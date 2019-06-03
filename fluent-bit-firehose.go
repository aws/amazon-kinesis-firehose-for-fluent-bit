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

package main

import (
	"C"
	"fmt"
	"unsafe"

	"github.com/awslabs/amazon-kinesis-firehose-for-fluent-bit/firehose"
	"github.com/fluent/fluent-bit-go/output"
)

var (
	out *firehose.FirehoseOutput
)

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "firehose", "Amazon Kinesis Firehose Fluent Bit Plugin!")
}

// (fluentbit will call this)
// ctx (context) pointer to fluentbit context (state/ c code)

//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {
	deliveryStream := output.FLBPluginConfigKey(ctx, "delivery-stream")
	fmt.Printf("[firehose] plugin parameter = '%s'\n", deliveryStream)
	region := output.FLBPluginConfigKey(ctx, "region")
	fmt.Printf("[firehose] plugin parameter = '%s'\n", region)
	dataKeys := output.FLBPluginConfigKey(ctx, "data_keys")
	fmt.Printf("[firehose] plugin parameter = '%s'\n", dataKeys)
	roleARN := output.FLBPluginConfigKey(ctx, "role_arn")
	fmt.Printf("[firehose] plugin parameter = '%s'\n", roleARN)

	if deliveryStream == "" || region == "" {
		return output.FLB_ERROR
	}

	var err error
	out, err = firehose.NewFirehoseOutput(region, deliveryStream, dataKeys, roleARN)
	if err != nil {
		fmt.Println(err)
		return output.FLB_ERROR
	}
	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	var count int
	var ret int
	var record map[interface{}]interface{}

	// Create Fluent Bit decoder
	dec := output.NewDecoder(data, int(length))

	fluentTag := C.GoString(tag)
	fmt.Printf("Found tag: %s\n", fluentTag)

	for {
		// Extract Record
		ret, _, record = output.GetRecord(dec)
		if ret != 0 {
			break
		}

		err := out.AddRecord(record)
		if err != nil {
			fmt.Println(err)
			// TODO: Better error handling
			return output.FLB_RETRY
		}
		count++
	}
	// Should we flush only FLBPluginExit? That would be more efficient but more risky
	err := out.Flush()
	if err != nil {
		fmt.Println(err)
		// TODO: Better error handling
		return output.FLB_RETRY
	}
	fmt.Printf("Processed %d events with tag %s\n", count, fluentTag)

	// Return options:
	//
	// output.FLB_OK    = data have been processed.
	// output.FLB_ERROR = unrecoverable error, do not try this again.
	// output.FLB_RETRY = retry to flush later.
	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}
