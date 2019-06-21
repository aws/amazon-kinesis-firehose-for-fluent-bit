## Fluent Bit Plugin for Amazon Kinesis Firehose

A Fluent Bit output plugin for Amazon Kinesis Data Firehose.

#### Security disclosures

If you think you’ve found a potential security issue, please do not post it in the Issues.  Instead, please follow the instructions [here](https://aws.amazon.com/security/vulnerability-reporting/) or email AWS security directly at [aws-security@amazon.com](mailto:aws-security@amazon.com).

### Usage

Run `make` to build `./bin/firehose.so`. Then use with Fluent Bit:
```
./fluent-bit -e ./firehose.so -i cpu \
-o firehose \
-p "region=us-west-2" \
-p "delivery-stream=example-stream"
```

### Plugin Options

* `region`: The region which your Firehose delivery stream(s) is/are in.
* `delivery_stream`: The name of the delivery stream that you want log records sent to.
* `data_keys`: By default, the whole log record will be sent to Kinesis. If you specify a key name(s) with this option, then only those keys and values will be sent to Kinesis. For example, if you are using the Fluentd Docker log driver, you can specify `data_keys log` and only the log message will be sent to Kinesis. If you specify multiple keys, they should be comma delimited.
* `role_arn`: ARN of an IAM role to assume (for cross account access).

### Permissions

The plugin requires `firehose:PutRecordBatch` permissions.

### Credentials

This plugin uses the AWS SDK Go, and uses its [default credential provider chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html). If you are using the plugin on Amazon EC2 or Amazon ECS, the plugin will use your EC2 instance role or ECS Task role permissions. The plugin can also retrieve credentials from a (shared credentials file)[https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html], or from the standard `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` environment variables. 

### Environment Variables

* `FLB_LOG_LEVEL`: Set the log level for Fluent Bit and the plugin. Valid values are: `debug`, `info`, and `error` (case insensitive). Default is `info`. **Note**: Setting log level in the Fluent Bit Configuration file using the Service key will not affect the plugin log level (because the plugin is external).
* `SEND_FAILURE_TIMEOUT`: Allows you to configure a timeout if the plugin can not send logs to Firehose. The timeout is specified as a [Golang duration](https://golang.org/pkg/time/#ParseDuration), for example: `5m30s`. If the plugin has failed to make any progress for the given period of time, then it will exit and kill Fluent Bit. This is useful in scenarios where you want your logging solution to fail fast if it has been misconfigured (i.e. network or credentials have not been set up to allow it to send to Firehose).

### Example Fluent Bit Config File

```
[INPUT]
    Name        forward
    Listen      0.0.0.0
    Port        24224

[OUTPUT]
    Name   firehose
    Match  *
    region us-west-2
    delivery_stream my-stream
```

## License

This library is licensed under the Apache 2.0 License.
