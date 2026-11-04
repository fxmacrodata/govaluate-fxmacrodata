# FXMacroData for Govaluate

Apply native Govaluate conditions and calculations to complete FXMacroData JSON responses. This library performs no HTTP calls and accepts no API key. Obtain responses with your own FXMacroData client and entitlement, then pass the complete JSON to the adapter.

```go
import fxmdgovaluate "github.com/fxmacrodata/govaluate-fxmacrodata"

result, err := fxmdgovaluate.EvaluateJSON(responseJSON,
    `field('/currency') == 'USD' && length('/data') > 0`)
// result.Value is the derived rule output; result.Raw retains the original JSON.
```

The entire response remains available: observations, revisions, nulls, provenance, pagination, quality flags, timestamps and unknown future fields. No rows are flattened, revisions collapsed, dates inferred, strings converted to numbers or missing fields fabricated. `Evaluate` also accepts JSON-serializable SDK models; use `EvaluateJSON` for original bytes and fields a typed model cannot retain.

## Native rule functions

| Function | Behaviour |
| --- | --- |
| `field('/data/0/value')` | Read a scalar as a native number, string, boolean or null. Missing fields are errors. |
| `exists('/data/0/value')` | Test presence independently of null. |
| `json('/data')` | Return a compact JSON view of any subtree, retaining number lexemes. The complete original response remains in `Raw`. |
| `length('/data')` | Count array items, object fields or Unicode string code points. |

Pointers use RFC 6901 escaping: `~1` for `/`, `~0` for `~`, and the empty pointer for the root. Array indices must be canonical nonnegative integers. Empty arrays/objects remain empty; null remains null. For example `exists('/value') && json('/value') == 'null'` tests an explicit null without treating a missing field as null.

Govaluate arithmetic uses binary64 numbers. Scalar numeric access and numeric literals are limited to the finite safe range of plus/minus 9,007,199,254,740,991; decimal arithmetic follows native floating-point semantics. Use `json('/identifier')` for exact large-integer or decimal text. Original numeric spelling is always retained in `Raw`. Implicit Govaluate date literals are disabled; timestamps in the response stay strings. Supply date-looking comparison strings as explicit JSON constants:

```go
result, err := fxmdgovaluate.EvaluateJSONWithConstants(responseJSON,
    `field('/date') == expected_date`,
    []byte(`{"expected_date":"2026-11-01"}`))
```

Constants are a JSON object of scalar values with identifier names. They are limited to 64 KiB and follow the same native numeric range. This optional rule configuration does not change response fields. Only these explicitly supplied constants and the four pointer functions are exposed; environment variables, Go object methods, application objects and custom functions are unavailable.

## Product and SDK compatibility

The response adapter is endpoint agnostic. Use every operation and original path/query/body parameter of the public Go SDK v0.1.0 or your standard public v1 HTTP client. There is no endpoint dispatch list, replacement FXMacroData schema, network proxy or automatic pagination. [CAPABILITIES.md](CAPABILITIES.md) records the JSON response families. The native tests use fictional payloads and check published SDK method availability; they do not call or certify live endpoints.

Rules are reviewed application configuration, not a hostile multi-tenant execution interface. Input is limited to 4 MiB, JSON depth to 64, values to 50,000, rules to 4,096 bytes and native tokens to 512. Aggregate materialized views and string results are limited to 8 MiB. Duplicate keys, invalid UTF-8, malformed JSON, invalid pointers, non-finite results and unsupported scalar conversions fail explicitly. Errors do not echo response bodies or rule text. The adapter provides no hard CPU deadline for the native engine.

`examples/rules` runs an explicitly fictional fixture offline. This release uses Govaluate v3.0.0 and the public Go SDK v0.1.0. It is an independent companion, with no upstream endorsement. [FXMacroData](https://fxmacrodata.com/?utm_source=github&utm_medium=integration&utm_campaign=open_source&utm_content=govaluate_fxm) supplies official-source macroeconomic data. Code is MIT licensed; API data access and redistribution rights remain governed by each customer's plan and agreement.
