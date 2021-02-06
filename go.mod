module github.com/frankfarrell/terraform-provider-redshift

go 1.12

require (
	github.com/hashicorp/terraform v0.12.2
	github.com/hashicorp/terraform-plugin-sdk/v2 v2.4.2
	github.com/lib/pq v1.1.1
)

replace git.apache.org/thrift.git => github.com/apache/thrift v0.0.0-20180902110319-2566ecd5d999
