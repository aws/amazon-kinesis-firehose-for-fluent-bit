[![Test Actions Status](https://github.com/aws/amazon-kinesis-firehose-for-fluent-bit/workflows/Build/badge.svg)](https://github.com/aws/amazon-kinesis-firehose-for-fluent-bit/actions)
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
* `endpoint`: Specify a custom endpoint for the Kinesis Firehose API.
* `sts_endpoint`: Specify a custom endpoint for the STS API; used to assume your custom role provided with `role_arn`.
* `time_key`: Add the timestamp to the record under this key. By default the timestamp from Fluent Bit will not be added to records sent to Kinesis.
* `time_key_format`: [strftime](http://man7.org/linux/man-pages/man3/strftime.3.html) compliant format string for the timestamp; for example, `%Y-%m-%dT%H:%M:%S%z`. This option is used with `time_key`. 

### Permissions

The plugin requires `firehose:PutRecordBatch` permissions.

### Credentials

This plugin uses the AWS SDK Go, and uses its [default credential provider chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html). If you are using the plugin on Amazon EC2 or Amazon ECS or Amazon EKS, the plugin will use your EC2 instance role or [ECS Task role permissions](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html) or [EKS IAM Roles for Service Accounts for pods](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html). The plugin can also retrieve credentials from a [shared credentials file](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html), or from the standard `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` environment variables.

### Environment Variables

* `FLB_LOG_LEVEL`: Set the log level for the plugin. Valid values are: `debug`, `info`, and `error` (case insensitive). Default is `info`. **Note**: Setting log level in the Fluent Bit Configuration file using the Service key will not affect the plugin log level (because the plugin is external).
* `SEND_FAILURE_TIMEOUT`: Allows you to configure a timeout if the plugin can not send logs to Firehose. The timeout is specified as a [Golang duration](https://golang.org/pkg/time/#ParseDuration), for example: `5m30s`. If the plugin has failed to make any progress for the given period of time, then it will exit and kill Fluent Bit. This is useful in scenarios where you want your logging solution to fail fast if it has been misconfigured (i.e. network or credentials have not been set up to allow it to send to Firehose).

### Fluent Bit Versions

This plugin has been tested with Fluent Bit 1.2.0+. It may not work with older Fluent Bit versions. We recommend using the latest version of Fluent Bit as it will contain the newest features and bug fixes.

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

### AWS for Fluent Bit

We distribute a container image with Fluent Bit and these plugins.

##### GitHub

[github.com/aws/aws-for-fluent-bit](https://github.com/aws/aws-for-fluent-bit)

##### Docker Hub

[amazon/aws-for-fluent-bit](https://hub.docker.com/r/amazon/aws-for-fluent-bit/tags)

##### Amazon ECR

You can use our SSM Public Parameters to find the Amazon ECR image URI in your region:

```
aws ssm get-parameters-by-path --path /aws/service/aws-for-fluent-bit/
```

For more see [our docs](https://github.com/aws/aws-for-fluent-bit#public-images).

## License

This library is licensed under the Apache 2.0 License.
