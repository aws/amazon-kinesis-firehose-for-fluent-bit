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

	"github.com/aws/amazon-kinesis-firehose-for-fluent-bit/firehose/mock_firehose"
	"github.com/aws/amazon-kinesis-firehose-for-fluent-bit/plugins"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	fluentbit "github.com/fluent/fluent-bit-go/output"
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
		timer:          timer,
	}

	record := map[interface{}]interface{}{
		"somekey": []byte("some value"),
	}

	timeStamp := time.Now()
	retCode := output.AddRecord(record, &timeStamp)

	assert.Equal(t, retCode, fluentbit.FLB_OK, "Expected return code to be FLB_OK")
	assert.Len(t, output.records, 1, "Expected output to contain 1 record")
}

func TestTruncateLargeLogEvent(t *testing.T) {
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
		timer:          timer,
	}

	record := map[interface{}]interface{}{
		"somekey": make([]byte, 1024005),
	}

	timeStamp := time.Now()
	retCode := output.AddRecord(record, &timeStamp)
	actualData, err := output.processRecord(record)

	if err != nil {
		logrus.Debugf("[firehose %d] Failed to marshal record: %v\n", output.PluginID, record)
	}

	assert.Equal(t, retCode, fluentbit.FLB_OK, "Expected return code to be FLB_OK")
	assert.Len(t, output.records, 1, "Expected output to contain 1 record")
	assert.Len(t, actualData, 1024000, "Expected length is 1024000")
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
		timer:          timer,
	}

	timeStamp := time.Now()
	retCode := output.AddRecord(record, &timeStamp)
	assert.Equal(t, retCode, fluentbit.FLB_OK, "Expected return code to be FLB_OK")

	retCode = output.Flush()
	assert.Equal(t, retCode, fluentbit.FLB_OK, "Expected return code to be FLB_OK")

}

func TestSendCurrentBatch(t *testing.T) {
	output := OutputPlugin{
		region:         "us-east-1",
		deliveryStream: "stream",
		dataKeys:       "",
		client:         nil,
		records:        nil,
	}

	retCode, err := output.sendCurrentBatch()

	assert.Equal(t, retCode, fluentbit.FLB_OK, "Expected return code to be FLB_OK")
	assert.Nil(t, err)

	output.records = make([]*firehose.Record, 0, 500)
	retCode, err = output.sendCurrentBatch()

	assert.Equal(t, retCode, fluentbit.FLB_OK, "Expected return code to be FLB_OK")
	assert.Nil(t, err)

}
