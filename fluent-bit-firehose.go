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
	"unsafe"

	"github.com/aws/amazon-kinesis-firehose-for-fluent-bit/firehose"
	"github.com/aws/amazon-kinesis-firehose-for-fluent-bit/plugins"
	"github.com/fluent/fluent-bit-go/output"

	"github.com/sirupsen/logrus"
)
import (
	"fmt"
	"time"
)

var (
	pluginInstances []*firehose.OutputPlugin
)

func addPluginInstance(ctx unsafe.Pointer) error {
	pluginID := len(pluginInstances)
	output.FLBPluginSetContext(ctx, pluginID)
	instance, err := newFirehoseOutput(ctx, pluginID)
	if err != nil {
		return err
	}

	pluginInstances = append(pluginInstances, instance)
	return nil
}

func getPluginInstance(ctx unsafe.Pointer) *firehose.OutputPlugin {
	pluginID := output.FLBPluginGetContext(ctx).(int)
	return pluginInstances[pluginID]
}

// The "export" comments have syntactic meaning
// This is how the compiler knows a function should be callable from the C code

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "firehose", "Amazon Kinesis Data Firehose Fluent Bit Plugin.")
}

func newFirehoseOutput(ctx unsafe.Pointer, pluginID int) (*firehose.OutputPlugin, error) {
	deliveryStream := output.FLBPluginConfigKey(ctx, "delivery_stream")
	logrus.Infof("[firehose %d] plugin parameter delivery_stream = '%s'\n", pluginID, deliveryStream)
	region := output.FLBPluginConfigKey(ctx, "region")
	logrus.Infof("[firehose %d] plugin parameter region = '%s'\n", pluginID, region)
	dataKeys := output.FLBPluginConfigKey(ctx, "data_keys")
	logrus.Infof("[firehose %d] plugin parameter data_keys = '%s'\n", pluginID, dataKeys)
	roleARN := output.FLBPluginConfigKey(ctx, "role_arn")
	logrus.Infof("[firehose %d] plugin parameter role_arn = '%s'\n", pluginID, roleARN)
	firehoseEndpoint := output.FLBPluginConfigKey(ctx, "endpoint")
	logrus.Infof("[firehose %d] plugin parameter endpoint = '%s'\n", pluginID, firehoseEndpoint)
	stsEndpoint := output.FLBPluginConfigKey(ctx, "sts_endpoint")
	logrus.Infof("[firehose %d] plugin parameter sts_endpoint = '%s'\n", pluginID, stsEndpoint)
	timeKey := output.FLBPluginConfigKey(ctx, "time_key")
	logrus.Infof("[firehose %d] plugin parameter time_key = '%s'\n", pluginID, timeKey)
	timeKeyFmt := output.FLBPluginConfigKey(ctx, "time_key_format")
	logrus.Infof("[firehose %d] plugin parameter time_key_format = '%s'\n", pluginID, timeKeyFmt)

	if deliveryStream == "" || region == "" {
		return nil, fmt.Errorf("[firehose %d] delivery_stream and region are required configuration parameters", pluginID)
	}

	return firehose.NewOutputPlugin(region, deliveryStream, dataKeys, roleARN, firehoseEndpoint, stsEndpoint, timeKey, timeKeyFmt, pluginID)
}

//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {
	plugins.SetupLogger()

	err := addPluginInstance(ctx)
	if err != nil {
		logrus.Errorf("[firehose] Failed to initialize plugin: %v\n", err)
		return output.FLB_ERROR
	}
	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	var count int
	var ret int
	var ts interface{}
	var timestamp time.Time
	var record map[interface{}]interface{}

	// Create Fluent Bit decoder
	dec := output.NewDecoder(data, int(length))

	firehoseOutput := getPluginInstance(ctx)
	fluentTag := C.GoString(tag)
	logrus.Debugf("[firehose %d] Found logs with tag: %s\n", firehoseOutput.PluginID, fluentTag)

	for {
		// Extract Record
		ret, ts, record = output.GetRecord(dec)
		if ret != 0 {
			break
		}

		switch tts := ts.(type) {
		case output.FLBTime:
			timestamp = tts.Time
		case uint64:
			// when ts is of type uint64 it appears to
			// be the amount of seconds since unix epoch.
			timestamp = time.Unix(int64(tts), 0)
		default:
			timestamp = time.Now()
		}

		retCode := firehoseOutput.AddRecord(record, &timestamp)
		if retCode != output.FLB_OK {
			return retCode
		}
		count++
	}
	retCode := firehoseOutput.Flush()
	if retCode != output.FLB_OK {
		return retCode
	}
	logrus.Debugf("[firehose %d] Processed %d events with tag %s\n", firehoseOutput.PluginID, count, fluentTag)

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	// Before final exit, call Flush() for all the instances of the Output Plugin
	for i := range pluginInstances {
		pluginInstances[i].Flush()
	}

	return output.FLB_OK
}

func main() {
}
