# FXMacroData response capabilities

This offline adapter applies native Govaluate rules to complete JSON values within the documented validation limits. Original response bytes remain separate from the rule result. It supplies no API transport or credentials. Customers use their own SDK/client and entitlement; original endpoint paths, query/body parameters, filters and pagination remain with that client.

| FXMacroData capability | Target platform surface | Auth model | Implemented files | Tests | Status |
| --- | --- | --- | --- | --- | --- |
| Catalogue/discovery | Native scalar/presence/container/JSON functions | Customer client; adapter has no credentials | govaluate.go | catalogue fixture | native response rules |
| Macro history/releases/revisions | Complete JSON and independent rule output | Customer client and API entitlement | govaluate.go | revision, null, clock and raw-byte tests | native response rules |
| Calendar | JSON Pointer functions over the complete response | Customer client and API entitlement | govaluate.go | calendar and explicit date-string constants | native response rules |
| Predictions/coverage | Full JSON without a replacement schema | Customer client and API entitlement | govaluate.go | predictions/coverage fixture | native response rules |
| Central-bank press releases | Full JSON with provenance | Customer client and API entitlement | govaluate.go | central-bank fixture | native response rules |
| FX and source/reference metadata | Full JSON with original number text in Raw/json views | Customer client and API entitlement | govaluate.go | FX/source and numeric-boundary fixtures | native response rules |
| COT/commodities | Full JSON and independent scalar rules | Customer client and API entitlement | govaluate.go | COT/commodity fixture | native response rules |
| Sessions/financial prices | Full JSON including empty collections | Customer client and API entitlement | govaluate.go | sessions/prices fixture | native response rules |
| Curves/factors/differentials/sentiment | Full JSON and native pointer functions | Customer client and API entitlement | govaluate.go | combined structured-response fixture | native response rules |
| Research panels | Nested JSON retained without flattening | Customer client and API entitlement | govaluate.go | research-panel fixture | native response rules |
| Other public v1 JSON, including seasonality/news where entitled | Generic JSON response input; no endpoint allowlist | Customer client and API entitlement | govaluate.go | future-field, array and scalar fixtures | native response rules |

These are native offline JSON-mechanism tests with fictional payloads, not individual live endpoint or customer-installation certifications. No missing values, release clocks, forecasts or future events are inferred. Empty responses and nulls stay intact; revisions are not collapsed. There is no automatic pagination, endpoint renaming, data-fetching fallback or date-window override. Streaming is outside this JSON processor.

`EvaluateJSON` retains original JSON bytes, including fields a typed model may not support. `Evaluate` serializes the caller's typed value and therefore preserves that model's JSON representation. `field` supplies supported native scalars, `exists` separates null from absence, `json` supplies compact exact-number JSON views, and `length` counts containers or Unicode code points. Govaluate arithmetic uses binary64; scalar access/literals have the documented safe-range boundary. Larger number text remains available through `json` and unchanged `Raw`.

Optional explicit JSON scalar constants preserve date-looking comparison strings. They are rule configuration and never change the response schema. Only the supplied scalar constants and four lookup functions are exposed; host environment variables, application objects, Go methods and custom functions are unavailable.

Public Go SDK v0.1.0 method availability is checked separately for all 27 operations: GetAnnouncementChanges, GetAnnouncements, GetCalendar, GetCommodities, GetCot, GetCurves, GetDataCatalogue, GetFactor, GetFinancialPrices, GetForex, GetFxIntradayReferenceRates, GetFxReferenceRates, GetFxSessionRates, GetFxSourceUniverse, GetFxSources, GetHealth, GetLatestAnnouncements, GetLatestCommodities, GetMarketSessions, GetPredictionCoverage, GetPredictionCoverageForCurrency, GetPredictions, GetPressReleases, GetRateDifferentials, GetRiskSentiment, Ping and ResearchPanel. These checks issue no requests. Every original SDK parameter remains available through the SDK; the adapter introduces no request parameter list.
