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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/awslabs/amazon-kinesis-firehose-for-fluent-bit/firehose/mock_firehose"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestAddRecord(t *testing.T) {
	output := FirehoseOutput{
		region:         "us-east-1",
		deliveryStream: "stream",
		dataKeys:       "",
		client:         nil,
		records:        make([]*firehose.Record, 0, 500),
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
	mockFirehose := mock_firehose.NewMockFirehoseClient(ctrl)

	mockFirehose.EXPECT().PutRecordBatch(gomock.Any()).Return(&firehose.PutRecordBatchOutput{
		FailedPutCount: aws.Int64(0),
	}, nil)

	output := FirehoseOutput{
		region:         "us-east-1",
		deliveryStream: "stream",
		dataKeys:       "",
		client:         mockFirehose,
		records:        make([]*firehose.Record, 0, 500),
	}

	output.AddRecord(record)
	output.Flush()

}
