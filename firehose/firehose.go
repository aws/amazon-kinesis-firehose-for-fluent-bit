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
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/aws/amazon-kinesis-firehose-for-fluent-bit/plugins"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	fluentbit "github.com/fluent/fluent-bit-go/output"
	jsoniter "github.com/json-iterator/go"
	"github.com/lestrrat-go/strftime"
	"github.com/sirupsen/logrus"
)

const (
	// Firehose API Limit https://docs.aws.amazon.com/firehose/latest/dev/limits.html
	maximumRecordsPerPut      = 500
	maximumPutRecordBatchSize = 4194304 // 4 MiB
	maximumRecordSize         = 1024000 // 1000 KiB
)

const (
	// We use strftime format specifiers because this will one day be re-written in C
	defaultTimeFmt = "%Y-%m-%dT%H:%M:%S"
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
	timeKey        string
	fmtStrftime    *strftime.Strftime
	client         PutRecordBatcher
	records        []*firehose.Record
	dataLength     int
	timer          *plugins.Timeout
	PluginID       int
}

// NewOutputPlugin creates an OutputPlugin object
func NewOutputPlugin(region, deliveryStream, dataKeys, roleARN, firehoseEndpoint, stsEndpoint, timeKey, timeFmt string, pluginID int) (*OutputPlugin, error) {
	client, err := newPutRecordBatcher(roleARN, region, firehoseEndpoint, stsEndpoint)
	if err != nil {
		return nil, err
	}

	records := make([]*firehose.Record, 0, maximumRecordsPerPut)

	timer, err := plugins.NewTimeout(func(d time.Duration) {
		logrus.Errorf("[firehose %d] timeout threshold reached: Failed to send logs for %s\n", pluginID, d.String())
		logrus.Errorf("[firehose %d] Quitting Fluent Bit", pluginID)
		os.Exit(1)
	})

	if err != nil {
		return nil, err
	}

	var timeFormatter *strftime.Strftime
	if timeKey != "" {
		if timeFmt == "" {
			timeFmt = defaultTimeFmt
		}
		timeFormatter, err = strftime.New(timeFmt, strftime.WithMilliseconds('L'), strftime.WithMicroseconds('f'))
		if err != nil {
			logrus.Errorf("[firehose %d] Issue with strftime format in 'time_key_format'", pluginID)
			return nil, err
		}
	}

	return &OutputPlugin{
		region:         region,
		deliveryStream: deliveryStream,
		client:         client,
		records:        records,
		dataKeys:       dataKeys,
		timer:          timer,
		timeKey:        timeKey,
		fmtStrftime:    timeFormatter,
		PluginID:       pluginID,
	}, nil
}

func newPutRecordBatcher(roleARN, region, firehoseEndpoint, stsEndpoint string) (*firehose.Firehose, error) {
	customResolverFn := func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
		if service == endpoints.FirehoseServiceID && firehoseEndpoint != "" {
			return endpoints.ResolvedEndpoint{
				URL: firehoseEndpoint,
			}, nil
		} else if service == endpoints.StsServiceID && stsEndpoint != "" {
			return endpoints.ResolvedEndpoint{
				URL: stsEndpoint,
			}, nil
		}
		return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
	}

	sess, err := session.NewSession(&aws.Config{
		Region:                        aws.String(region),
		EndpointResolver:              endpoints.ResolverFunc(customResolverFn),
		CredentialsChainVerboseErrors: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}

	svcConfig := &aws.Config{}
	if roleARN != "" {
		creds := stscreds.NewCredentials(sess, roleARN)
		svcConfig.Credentials = creds
	}

	client := firehose.New(sess, svcConfig)
	client.Handlers.Build.PushBackNamed(plugins.CustomUserAgentHandler())
	return client, nil
}

// AddRecord accepts a record and adds it to the buffer, flushing the buffer if it is full
// the return value is one of: FLB_OK FLB_RETRY
// API Errors lead to an FLB_RETRY, and all other errors are logged, the record is discarded and FLB_OK is returned
func (output *OutputPlugin) AddRecord(record map[interface{}]interface{}, timeStamp *time.Time) int {
	if output.timeKey != "" {
		buf := new(bytes.Buffer)
		err := output.fmtStrftime.Format(buf, *timeStamp)
		if err != nil {
			logrus.Errorf("[firehose %d] Could not create timestamp %v\n", output.PluginID, err)
			return fluentbit.FLB_ERROR
		}
		record[output.timeKey] = buf.String()
	}
	data, err := output.processRecord(record)
	if err != nil {
		logrus.Errorf("[firehose %d] %v\n", output.PluginID, err)
		// discard this single bad record instead and let the batch continue
		return fluentbit.FLB_OK
	}

	newDataSize := len(data)

	if len(output.records) == maximumRecordsPerPut || (output.dataLength+newDataSize) > maximumPutRecordBatchSize {
		retCode, err := output.sendCurrentBatch()
		if err != nil {
			logrus.Errorf("[firehose %d] %v\n", output.PluginID, err)
		}
		if retCode != fluentbit.FLB_OK {
			return retCode
		}
	}

	output.records = append(output.records, &firehose.Record{
		Data: data,
	})
	output.dataLength += newDataSize
	return fluentbit.FLB_OK
}

// Flush sends the current buffer of records
// Returns FLB_OK, FLB_RETRY, FLB_ERROR
func (output *OutputPlugin) Flush() int {
	retCode, err := output.sendCurrentBatch()
	if err != nil {
		logrus.Errorf("[firehose %d] %v\n", output.PluginID, err)
	}
	return retCode
}

func (output *OutputPlugin) processRecord(record map[interface{}]interface{}) ([]byte, error) {
	if output.dataKeys != "" {
		record = plugins.DataKeys(output.dataKeys, record)
	}

	var err error
	record, err = plugins.DecodeMap(record)
	if err != nil {
		logrus.Debugf("[firehose %d] Failed to decode record: %v\n", output.PluginID, record)
		return nil, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.Marshal(record)
	if err != nil {
		logrus.Debugf("[firehose %d] Failed to marshal record: %v\n", output.PluginID, record)
		return nil, err
	}

	// append newline
	data = append(data, []byte("\n")...)

	if len(data) > maximumRecordSize {
		return nil, fmt.Errorf("Log record greater than max size allowed by Kinesis")
	}

	return data, nil
}

func (output *OutputPlugin) sendCurrentBatch() (int, error) {
	output.timer.Check()

	response, err := output.client.PutRecordBatch(&firehose.PutRecordBatchInput{
		DeliveryStreamName: aws.String(output.deliveryStream),
		Records:            output.records,
	})
	if err != nil {
		logrus.Errorf("[firehose %d] PutRecordBatch failed with %v", output.PluginID, err)
		output.timer.Start()
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == firehose.ErrCodeServiceUnavailableException {
				logrus.Warnf("[firehose %d] Throughput limits for the delivery stream may have been exceeded.", output.PluginID)
			}
		}
		return fluentbit.FLB_RETRY, err
	}
	logrus.Debugf("[firehose %d] Sent %d events to Firehose\n", output.PluginID, len(output.records))

	return output.processAPIResponse(response)
}

// processAPIResponse processes the successful and failed records
// it returns an error iff no records succeeded (i.e.) no progress has been made
func (output *OutputPlugin) processAPIResponse(response *firehose.PutRecordBatchOutput) (int, error) {
	if aws.Int64Value(response.FailedPutCount) > 0 {
		// start timer if all records failed (no progress has been made)
		if aws.Int64Value(response.FailedPutCount) == int64(len(output.records)) {
			output.timer.Start()
			return fluentbit.FLB_RETRY, fmt.Errorf("PutRecordBatch request returned with no records successfully recieved")
		}

		logrus.Warnf("[firehose %d] %d records failed to be delivered. Will retry.\n", output.PluginID, aws.Int64Value(response.FailedPutCount))
		failedRecords := make([]*firehose.Record, 0, aws.Int64Value(response.FailedPutCount))
		// try to resend failed records
		for i, record := range response.RequestResponses {
			if record.ErrorMessage != nil {
				logrus.Debugf("[firehose %d] Record failed to send with error: %s\n", output.PluginID, aws.StringValue(record.ErrorMessage))
				failedRecords = append(failedRecords, output.records[i])
			}
			if aws.StringValue(record.ErrorCode) == firehose.ErrCodeServiceUnavailableException {
				logrus.Warnf("[firehose %d] Throughput limits for the delivery stream may have been exceeded.", output.PluginID)
				return fluentbit.FLB_RETRY, nil
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
		output.records = output.records[:0]
		output.dataLength = 0
	}

	return fluentbit.FLB_OK, nil
}
