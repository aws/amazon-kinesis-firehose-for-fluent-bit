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

// Package plugins contains functions that are useful across fluent bit plugins.
// This package will be imported by the CloudWatch Logs and Kinesis Data Streams plugins.
package plugins

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

const fluentBitLogLevelEnvVar = "FLB_LOG_LEVEL"

func SetupLogger() {
	logrus.SetOutput(os.Stdout)
	switch strings.ToUpper(os.Getenv(fluentBitLogLevelEnvVar)) {
	default:
		logrus.SetLevel(logrus.InfoLevel)
	case "DEBUG":
		logrus.SetLevel(logrus.DebugLevel)
	case "INFO":
		logrus.SetLevel(logrus.InfoLevel)
	case "ERROR":
		logrus.SetLevel(logrus.ErrorLevel)
	}
}

// []byte will be base64 encoded when marshaled to JSON, so we must directly cast all []byte to string
func DecodeMap(record map[interface{}]interface{}) (map[interface{}]interface{}, error) {
	for k, v := range record {
		switch t := v.(type) {
		case []byte:
			// convert all byte slices to strings
			record[k] = string(t)
		case map[interface{}]interface{}:
			decoded, err := DecodeMap(t)
			if err != nil {
				return nil, err
			}
			record[k] = decoded
		case []interface{}:
			decoded, err := decodeSlice(t)
			if err != nil {
				return nil, err
			}
			record[k] = decoded
		}
	}
	return record, nil
}

// DataKeys allows users to specify a list of keys in the record which they want to be sent
// all others are discarded
func DataKeys(input string, record map[interface{}]interface{}) map[interface{}]interface{} {
	input = strings.TrimSpace(input)
	keys := strings.Split(input, ",")

	for k := range record {
		var currentKey string
		switch t := k.(type) {
		case []byte:
			currentKey = string(t)
		case string:
			currentKey = t
		default:
			logrus.Debugf("[external plugin]: Unable to determine type of key %v\n", t)
			continue
		}

		if !contains(keys, currentKey) {
			delete(record, k)
		}
	}

	return record
}

func decodeSlice(record []interface{}) ([]interface{}, error) {
	for i, v := range record {
		switch t := v.(type) {
		case []byte:
			// convert all byte slices to strings
			record[i] = string(t)
		case map[interface{}]interface{}:
			decoded, err := DecodeMap(t)
			if err != nil {
				return nil, err
			}
			record[i] = decoded
		case []interface{}:
			decoded, err := decodeSlice(t)
			if err != nil {
				return nil, err
			}
			record[i] = decoded
		}
	}
	return record, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
