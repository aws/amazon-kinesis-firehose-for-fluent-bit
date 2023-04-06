# Changelog

## 1.7.2
* Enhancement - upgrade Go version to 1.20

## 1.7.1
* Enhancement - Added different base user agent for Linux and Windows

## 1.7.0
* Feature - Add support for building this plugin on Windows. *Note that this is only support in this plugin repo for Windows compilation.*

## 1.6.1
* Enhancement - upgrade Go version to 1.17

## 1.6.0
* Feature - Add new option `simple_aggregation` to send multiple log events per record (#12)

## 1.5.0
* Feature - Add new option `replace_dots` to replace dots in key names (#46)

## 1.4.2
* Bug - Truncate record to max size (#58)

## 1.4.0
* Feature - Add `log_key` option for firehose output plugin (#33)
* Bug - Check for empty batch before sending (#27)

## 1.3.0
* Feature - Add `sts_endpoint` param for custom STS API endpoint (#31)

## 1.2.1
* Bug - Remove exponential backoff code (#23)

## 1.2.0
* Feature - Add `time_key` and `time_key_format` config options to add timestamp to records (#9)

## 1.1.0
* Feature - Support IAM Roles for Service Accounts in Amazon EKS (#17)
* Enhancement - Change the log severity from `error` to `warning` for retryable API errors (#18)


## 1.0.0
Initial versioned release of the Amazon Kinesis Data Firehose for Fluent Bit Plugin
