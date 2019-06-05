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
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/awslabs/amazon-kinesis-firehose-for-fluent-bit/firehose/mock_firehose"
	"github.com/awslabs/amazon-kinesis-firehose-for-fluent-bit/plugins"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestAddRecord(t *testing.T) {
	timer, _ := plugins.NewTimeout(func(d time.Duration) {
		logrus.Errorf("[firehose] timeout threshold reached: Failed to send logs for %v\n", d)
		logrus.Error("[firehose] Quitting Fluent Bit")
		os.Exit(1)
	})
	output := OutputPlugin{
		region:         "us-east-1",
		deliveryStream: "stream",
		dataKeys:       "",
		client:         nil,
		records:        make([]*firehose.Record, 0, 500),
		backoff:        plugins.NewBackoff(),
		timer:          timer,
	}

	record := map[interface{}]interface{}{
		"somekey": []byte("some value"),
	}

	output.AddRecord(record)

	assert.Len(t, output.records, 1, "Expected output to contain 1 record")
}

func TestAddRecordAndFlush(t *testing.T) {
	record := map[interface{}]interface{}{
		"somekey": []byte("some value"),
	}

	ctrl := gomock.NewController(t)
	mockFirehose := mock_firehose.NewMockPutRecordBatcher(ctrl)

	mockFirehose.EXPECT().PutRecordBatch(gomock.Any()).Return(&firehose.PutRecordBatchOutput{
		FailedPutCount: aws.Int64(0),
	}, nil)

	timer, _ := plugins.NewTimeout(func(d time.Duration) {
		logrus.Errorf("[firehose] timeout threshold reached: Failed to send logs for %v\n", d)
		logrus.Error("[firehose] Quitting Fluent Bit")
		os.Exit(1)
	})

	output := OutputPlugin{
		region:         "us-east-1",
		deliveryStream: "stream",
		dataKeys:       "",
		client:         mockFirehose,
		records:        make([]*firehose.Record, 0, 500),
		backoff:        plugins.NewBackoff(),
		timer:          timer,
	}

	output.AddRecord(record)
	output.Flush()

}
