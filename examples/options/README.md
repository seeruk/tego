# Options

This example highlights many of Tego's available options: generated names and comments, enum
underlying values, struct tags, omitted fields, nullable fields, omittable fields, preserved integer
widths, and service/method option syntax.

It also demonstrates automatic file-level JSON tags, a message-level casing override, disabling the
file default for one message, and explicit field-level JSON tag overrides.

This example uses `rpc=none`, so the service and method options are present as schema examples only.

Good files to start with:

- `proto/options/v1/options.proto`
- `options/options.tego.go`

From this example folder, regenerate with:

```sh
buf generate
```
