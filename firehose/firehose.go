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

package firehose

import (
	"C"
)
import (
	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/awslabs/amazon-kinesis-firehose-for-fluent-bit/plugins"
	jsoniter "github.com/json-iterator/go"
)

const (
	maximumRecordsPerPut = 500
)

type FirehoseClient interface {
	PutRecordBatch(input *firehose.PutRecordBatchInput) (*firehose.PutRecordBatchOutput, error)
}

type FirehoseOutput struct {
	region         string
	deliveryStream string
	dataKeys       string
	client         FirehoseClient
	records        []*firehose.Record
}

func NewFirehoseOutput(region string, deliveryStream string, dataKeys string, roleARN string) (*FirehoseOutput, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, err
	}

	client := firehoseClient(roleARN, sess)

	records := make([]*firehose.Record, 0, 500)

	return &FirehoseOutput{
		region:         region,
		deliveryStream: deliveryStream,
		client:         client,
		records:        records,
		dataKeys:       dataKeys,
	}, nil
}

func firehoseClient(roleARN string, sess *session.Session) FirehoseClient {
	if roleARN != "" {
		creds := stscreds.NewCredentials(sess, roleARN)
		return firehose.New(sess, &aws.Config{Credentials: creds})
	}

	return firehose.New(sess)
}

// TODO: May be should re-write this to handle errors more carefully
// may be discard records that can't be marshaled?
// TODO: Look at how fluentd kinesis plugin does things
func (output *FirehoseOutput) AddRecord(record map[interface{}]interface{}) error {
	if output.dataKeys != "" {
		record = plugins.DataKeys(output.dataKeys, record)
	}

	var err error
	record, err = plugins.DecodeMap(record)
	if err != nil {
		return err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(output.records) == maximumRecordsPerPut {
		err = output.sendCurrentBatch()
		if err != nil {
			return err
		}
	}

	output.records = append(output.records, &firehose.Record{
		Data: data,
	})
	return nil
}

func (output *FirehoseOutput) Flush() error {
	return output.sendCurrentBatch()
}

func (output *FirehoseOutput) sendCurrentBatch() error {
	response, err := output.client.PutRecordBatch(&firehose.PutRecordBatchInput{
		DeliveryStreamName: aws.String(output.deliveryStream),
		Records:            output.records,
	})
	if err != nil {
		logrus.Error(err)
		return err
	}
	Logrus.Debugf("Sent %d events to Firehose\n", len(output.records))
	output.processAPIResponse(response)

	return nil
}

func (output *FirehoseOutput) processAPIResponse(response *firehose.PutRecordBatchOutput) {
	if aws.Int64Value(response.FailedPutCount) > 0 {
		logrus.Errorf("%d records failed to be delivered\n", aws.Int64Value(response.FailedPutCount))
		failedRecords := make([]*firehose.Record, 0, aws.Int64Value(response.FailedPutCount))
		// try to resend failed records
		for i, record := range response.RequestResponses {
			if record.ErrorMessage != nil {
				logrus.Debugf("Record failed to send with error: %s\n", aws.StringValue(record.ErrorMessage))
				failedRecords = append(failedRecords, output.records[i])
			}
		}

		output.records = output.records[:0]
		output.records = append(output.records, failedRecords...)

	} else {
		output.records = output.records[:0]
	}

}
