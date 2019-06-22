// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package firehose containers the OutputPlugin which sends log records to Firehose
package firehose

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/awslabs/amazon-kinesis-firehose-for-fluent-bit/plugins"
	fluentbit "github.com/fluent/fluent-bit-go/output"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
)

const (
	// Firehose API Limit https://docs.aws.amazon.com/firehose/latest/dev/limits.html
	maximumRecordsPerPut      = 500
	maximumPutRecordBatchSize = 4194304 // 4 MiB
	maximumRecordSize         = 1024000 // 1000 KiB
)

// PutRecordBatcher contains the firehose PutRecordBatch method call
type PutRecordBatcher interface {
	PutRecordBatch(input *firehose.PutRecordBatchInput) (*firehose.PutRecordBatchOutput, error)
}

// OutputPlugin sends log records to firehose
type OutputPlugin struct {
	region         string
	deliveryStream string
	dataKeys       string
	client         PutRecordBatcher
	records        []*firehose.Record
	dataLength     int
	backoff        *plugins.Backoff
	timer          *plugins.Timeout
}

// NewOutputPlugin creates a OutputPlugin object
func NewOutputPlugin(region, deliveryStream, dataKeys, roleARN, endpoint string) (*OutputPlugin, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, err
	}

	client := newPutRecordBatcher(roleARN, sess, endpoint)

	records := make([]*firehose.Record, 0, maximumRecordsPerPut)

	timer, err := plugins.NewTimeout(func(d time.Duration) {
		logrus.Errorf("[firehose] timeout threshold reached: Failed to send logs for %v\n", d)
		logrus.Error("[firehose] Quitting Fluent Bit")
		os.Exit(1)
	})

	if err != nil {
		return nil, err
	}

	return &OutputPlugin{
		region:         region,
		deliveryStream: deliveryStream,
		client:         client,
		records:        records,
		dataKeys:       dataKeys,
		backoff:        plugins.NewBackoff(),
		timer:          timer,
	}, nil
}

func newPutRecordBatcher(roleARN string, sess *session.Session, endpoint string) *firehose.Firehose {
	svcConfig := &aws.Config{}
	if endpoint != "" {
		defaultResolver := endpoints.DefaultResolver()
		cwCustomResolverFn := func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
			if service == "firehose" {
				return endpoints.ResolvedEndpoint{
					URL: endpoint,
				}, nil
			}
			return defaultResolver.EndpointFor(service, region, optFns...)
		}
		svcConfig.EndpointResolver = endpoints.ResolverFunc(cwCustomResolverFn)
	}

	if roleARN != "" {
		creds := stscreds.NewCredentials(sess, roleARN)
		svcConfig.Credentials = creds
	}

	return firehose.New(sess, svcConfig)
}

// AddRecord accepts a record and adds it to the buffer, flushing the buffer if it is full
// the return value is one of: FLB_OK FLB_RETRY
// API Errors lead to an FLB_RETRY, and all other errors are logged, the record is discarded and FLB_OK is returned
func (output *OutputPlugin) AddRecord(record map[interface{}]interface{}) int {
	data, err := output.processRecord(record)
	if err != nil {
		logrus.Errorf("[firehose] %v\n", err)
		// discard this single bad record instead and let the batch continue
		return fluentbit.FLB_OK
	}

	newDataSize := len(data)

	if len(output.records) == maximumRecordsPerPut || (output.dataLength+newDataSize) > maximumPutRecordBatchSize {
		err = output.sendCurrentBatch()
		if err != nil {
			logrus.Errorf("[firehose] %v\n", err)
			// send failures are retryable
			return fluentbit.FLB_RETRY
		}
	}

	output.records = append(output.records, &firehose.Record{
		Data: data,
	})
	output.dataLength += newDataSize
	return fluentbit.FLB_OK
}

// Flush sends the current buffer of records
func (output *OutputPlugin) Flush() error {
	return output.sendCurrentBatch()
}

func (output *OutputPlugin) processRecord(record map[interface{}]interface{}) ([]byte, error) {
	if output.dataKeys != "" {
		record = plugins.DataKeys(output.dataKeys, record)
	}

	var err error
	record, err = plugins.DecodeMap(record)
	if err != nil {
		logrus.Debugf("[firehose] Failed to decode record: %v\n", record)
		return nil, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.Marshal(record)
	if err != nil {
		logrus.Debugf("[firehose] Failed to marshal record: %v\n", record)
		return nil, err
	}

	// append newline
	data = append(data, []byte("\n")...)

	if len(data) > maximumRecordSize {
		return nil, fmt.Errorf("Log record greater than max size allowed by Kinesis")
	}

	return data, nil
}

func (output *OutputPlugin) sendCurrentBatch() error {
	output.backoff.Wait()
	output.timer.Check()

	response, err := output.client.PutRecordBatch(&firehose.PutRecordBatchInput{
		DeliveryStreamName: aws.String(output.deliveryStream),
		Records:            output.records,
	})
	if err != nil {
		logrus.Errorf("[firehose] PutRecordBatch failed with %v", err)
		output.timer.Start()
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == firehose.ErrCodeServiceUnavailableException {
				logrus.Warn("[firehose] Throughput limits for the delivery stream may have been exceeded.")
				// https://docs.aws.amazon.com/sdk-for-go/api/service/firehose/#Firehose.PutRecordBatch
				// Firehose recommends backoff when this error is encountered
				output.backoff.StartBackoff()
			}
		}
		return err
	}
	logrus.Debugf("[firehose] Sent %d events to Firehose\n", len(output.records))

	return output.processAPIResponse(response)
}

// processAPIResponse processes the successful and failed records
// it returns an error iff no records succeeded (i.e.) no progress has been made
func (output *OutputPlugin) processAPIResponse(response *firehose.PutRecordBatchOutput) error {
	if aws.Int64Value(response.FailedPutCount) > 0 {
		// start timer if all records failed (no progress has been made)
		if aws.Int64Value(response.FailedPutCount) == int64(len(output.records)) {
			output.timer.Start()
			return fmt.Errorf("PutRecordBatch request returned with no records successfully recieved")
		}

		logrus.Errorf("[firehose] %d records failed to be delivered\n", aws.Int64Value(response.FailedPutCount))
		failedRecords := make([]*firehose.Record, 0, aws.Int64Value(response.FailedPutCount))
		// try to resend failed records
		for i, record := range response.RequestResponses {
			if record.ErrorMessage != nil {
				logrus.Debugf("[firehose] Record failed to send with error: %s\n", aws.StringValue(record.ErrorMessage))
				failedRecords = append(failedRecords, output.records[i])
			}
			if aws.StringValue(record.ErrorCode) == firehose.ErrCodeServiceUnavailableException {
				// https://docs.aws.amazon.com/sdk-for-go/api/service/firehose/#Firehose.PutRecordBatch
				// Firehose recommends backoff when this error is encountered
				output.backoff.StartBackoff()
			}
		}

		output.records = output.records[:0]
		output.records = append(output.records, failedRecords...)
		output.dataLength = 0
		for _, record := range output.records {
			output.dataLength += len(record.Data)
		}

	} else {
		// request fully succeeded
		output.timer.Reset()
		output.backoff.Reset()
		output.records = output.records[:0]
		output.dataLength = 0
	}

	return nil
}
