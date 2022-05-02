[![Test Actions Status](https://github.com/aws/amazon-kinesis-firehose-for-fluent-bit/workflows/Build/badge.svg)](https://github.com/aws/amazon-kinesis-firehose-for-fluent-bit/actions)
## Fluent Bit Plugin for Amazon Kinesis Firehose

**NOTE: A new higher performance Fluent Bit Firehose Plugin has been released.** Check out our [official guidance](#new-higher-performance-core-fluent-bit-plugin).

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
* `log_key`: By default, the whole log record will be sent to Firehose. If you specify a key name with this option, then only the value of that key will be sent to Firehose. For example, if you are using the Fluentd Docker log driver, you can specify `log_key log` and only the log message will be sent to Firehose.
* `role_arn`: ARN of an IAM role to assume (for cross account access).
* `endpoint`: Specify a custom endpoint for the Kinesis Firehose API.
* `sts_endpoint`: Specify a custom endpoint for the STS API; used to assume your custom role provided with `role_arn`.
* `time_key`: Add the timestamp to the record under this key. By default the timestamp from Fluent Bit will not be added to records sent to Kinesis.
* `time_key_format`: [strftime](http://man7.org/linux/man-pages/man3/strftime.3.html) compliant format string for the timestamp; for example, `%Y-%m-%dT%H:%M:%S%z`. This option is used with `time_key`. You can also use `%L` for milliseconds and `%f` for microseconds. If you are using ECS FireLens, make sure you are running Amazon ECS Container Agent v1.42.0 or later, otherwise the timestamps associated with your container logs will only have second precision.
* `replace_dots`: Replace dot characters in key names with the value of this option. For example, if you add `replace_dots _` in your config then all occurrences of `.` will be replaced with an underscore. By default, dots will not be replaced.
* `simple_aggregation`: Option to allow plugin send multiple log events in the same record if current record not exceed the maximumRecordSize (1 MiB). It joins together as many log records as possible into a single Firehose record and delimits them with newline. It's good to enable if your destination supports aggregation like S3. Default to be `false`, set to `true` to enable this option.
### Permissions

The plugin requires `firehose:PutRecordBatch` permissions.

### Credentials

This plugin uses the AWS SDK Go, and uses its [default credential provider chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html). If you are using the plugin on Amazon EC2 or Amazon ECS or Amazon EKS, the plugin will use your EC2 instance role or [ECS Task role permissions](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html) or [EKS IAM Roles for Service Accounts for pods](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html). The plugin can also retrieve credentials from a [shared credentials file](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html), or from the standard `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` environment variables.

### Environment Variables

* `FLB_LOG_LEVEL`: Set the log level for the plugin. Valid values are: `debug`, `info`, and `error` (case insensitive). Default is `info`. **Note**: Setting log level in the Fluent Bit Configuration file using the Service key will not affect the plugin log level (because the plugin is external).
* `SEND_FAILURE_TIMEOUT`: Allows you to configure a timeout if the plugin can not send logs to Firehose. The timeout is specified as a [Golang duration](https://golang.org/pkg/time/#ParseDuration), for example: `5m30s`. If the plugin has failed to make any progress for the given period of time, then it will exit and kill Fluent Bit. This is useful in scenarios where you want your logging solution to fail fast if it has been misconfigured (i.e. network or credentials have not been set up to allow it to send to Firehose).


### New Higher Performance Core Fluent Bit Plugin

In the summer of 2020, we released a [new higher performance Kinesis Firehose plugin](https://docs.fluentbit.io/manual/pipeline/outputs/firehose) named `kinesis_firehose`.

That plugin has almost all of the features of this older, lower performance and less efficient plugin. Check out its [documentation](https://docs.fluentbit.io/manual/pipeline/outputs/firehose).

#### Do you plan to deprecate this older plugin?

This plugin will continue to be supported. However, we are pausing development on it and will focus on the high performance version instead.

#### Which plugin should I use?

If the features of the higher performance plugin are sufficient for your use cases, please use it. It can achieve higher throughput and will consume less CPU and memory.

As time goes on we expect new features to be added to the C plugin only, however, this is determined on a case by case basis. There is a small feature gap between the two plugins. Please consult the [C plugin documentation](https://docs.fluentbit.io/manual/pipeline/outputs/firehose) and this document for the features offered by each plugin. 

#### How can I migrate to the higher performance plugin?

For many users, you can simply replace the plugin name `firehose` with the new name `kinesis_firehose`. At the time of writing, the only feature missing from the high performance version is the `replace_dots` option. Check out its [documentation](https://docs.fluentbit.io/manual/pipeline/outputs/cloudwatch).

#### Do you accept contributions to both plugins?

Yes. The high performance plugin is written in C, and this plugin is written in Golang. We understand that Go is an easier language for amateur contributors to write code in- that is the primary reason we are continuing to maintain this repo.

However, if you can write code in C, please consider contributing new features to the [higher performance plugin](https://github.com/fluent/fluent-bit/tree/master/plugins/out_kinesis_firehose).

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
    replace_dots _
```

### AWS for Fluent Bit

We distribute a container image with Fluent Bit and these plugins.

##### GitHub

[github.com/aws/aws-for-fluent-bit](https://github.com/aws/aws-for-fluent-bit)

##### Amazon ECR Public Gallery

[aws-for-fluent-bit](https://gallery.ecr.aws/aws-observability/aws-for-fluent-bit)

Our images are available in Amazon ECR Public Gallery. You can download images with different tags by following command:

```
docker pull public.ecr.aws/aws-observability/aws-for-fluent-bit:<tag>
```

For example, you can pull the image with latest version by:

```
docker pull public.ecr.aws/aws-observability/aws-for-fluent-bit:latest
```

If you see errors for image pull limits, try log into public ECR with your AWS credentials:

```
aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin public.ecr.aws
```

You can check the [Amazon ECR Public official doc](https://docs.aws.amazon.com/AmazonECR/latest/public/get-set-up-for-amazon-ecr.html) for more details.

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
