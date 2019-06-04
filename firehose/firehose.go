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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
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
}

// NewOutputPlugin creates a OutputPlugin object
func NewOutputPlugin(region string, deliveryStream string, dataKeys string, roleARN string) (*OutputPlugin, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, err
	}

	client := newPutRecordBatcher(roleARN, sess)

	records := make([]*firehose.Record, 0, 500)

	return &OutputPlugin{
		region:         region,
		deliveryStream: deliveryStream,
		client:         client,
		records:        records,
		dataKeys:       dataKeys,
		backoff:        plugins.NewBackoff(),
	}, nil
}

func newPutRecordBatcher(roleARN string, sess *session.Session) PutRecordBatcher {
	if roleARN != "" {
		creds := stscreds.NewCredentials(sess, roleARN)
		return firehose.New(sess, &aws.Config{Credentials: creds})
	}

	return firehose.New(sess)
}

// AddRecord accepts a record and adds it to the buffer, flushing the buffer if it is full
func (output *OutputPlugin) AddRecord(record map[interface{}]interface{}) (int, error) {
	data, retCode, err := output.processRecord(record)
	if err != nil {
		return retCode, err
	}

	newDataSize := len(data)

	if len(output.records) == maximumRecordsPerPut || (output.dataLength+newDataSize) > maximumPutRecordBatchSize {
		err = output.sendCurrentBatch()
		if err != nil {
			return fluentbit.FLB_RETRY, err
		}
	}

	output.records = append(output.records, &firehose.Record{
		Data: data,
	})
	output.dataLength += newDataSize
	return fluentbit.FLB_RETRY, nil
}

// Flush sends the current buffer of records
func (output *OutputPlugin) Flush() error {
	return output.sendCurrentBatch()
}

func (output *OutputPlugin) processRecord(record map[interface{}]interface{}) ([]byte, int, error) {
	if output.dataKeys != "" {
		record = plugins.DataKeys(output.dataKeys, record)
	}

	var err error
	record, err = plugins.DecodeMap(record)
	if err != nil {
		logrus.Debugf("[firehose] Failed to decode record: %v\n", record)
		return nil, fluentbit.FLB_ERROR, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.Marshal(record)
	if err != nil {
		logrus.Debugf("[firehose] Failed to marshal record: %v\n", record)
		return nil, fluentbit.FLB_ERROR, err
	}

	if len(data) > maximumRecordSize {
		logrus.Errorf("[firehose] Discarding log record that is greater than %d bytes", maximumRecordSize)
		return nil, fluentbit.FLB_ERROR, fmt.Errorf("Log record greater than max size allowed by Kinesis")
	}

	return data, fluentbit.FLB_OK, nil
}

func (output *OutputPlugin) sendCurrentBatch() error {
	output.backoff.Wait()

	response, err := output.client.PutRecordBatch(&firehose.PutRecordBatchInput{
		DeliveryStreamName: aws.String(output.deliveryStream),
		Records:            output.records,
	})
	if err != nil {
		logrus.Errorf("[firehose] PutRecordBatch failed with %v", err)
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == firehose.ErrCodeServiceUnavailableException {
				logrus.Warn("[firehose] Throughput limits for the delivery stream may have been exceeded.")
				output.backoff.StartBackoff()
			}
		}
		return err
	}
	logrus.Debugf("[firehose] Sent %d events to Firehose\n", len(output.records))
	output.processAPIResponse(response)

	return nil
}

func (output *OutputPlugin) processAPIResponse(response *firehose.PutRecordBatchOutput) {
	if aws.Int64Value(response.FailedPutCount) > 0 {
		logrus.Errorf("[firehose] %d records failed to be delivered\n", aws.Int64Value(response.FailedPutCount))
		failedRecords := make([]*firehose.Record, 0, aws.Int64Value(response.FailedPutCount))
		// try to resend failed records
		for i, record := range response.RequestResponses {
			if record.ErrorMessage != nil {
				logrus.Debugf("[firehose] Record failed to send with error: %s\n", aws.StringValue(record.ErrorMessage))
				failedRecords = append(failedRecords, output.records[i])
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
		output.backoff.Reset()
		output.records = output.records[:0]
		output.dataLength = 0
	}

}
