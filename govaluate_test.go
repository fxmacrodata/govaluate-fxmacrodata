package fxmdgovaluate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Knetic/govaluate"
	sdk "github.com/fxmacrodata/fxmacrodata-go"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestNativeRulesKeepCompleteResponseAndOwnership(t *testing.T) {
	raw := []byte(` {"currency":"USD","data":[{"value":1,"revision":0},{"value":2,"revision":1},{"value":null,"revision":2}],"future":{"kept":true},"publication_time_status":"unknown"} `)
	result, err := EvaluateJSON(raw, `field('/currency') == 'USD' && length('/data') == 3 && field('/data/1/value') > 1 && exists('/data/2/value') && json('/data/2/value') == 'null'`)
	if err != nil || result.Value != true || !bytes.Equal(result.Raw, raw) {
		t.Fatal(result, err)
	}
	raw[0] = 'X'
	if result.Raw[0] == 'X' {
		t.Fatal("result aliases input")
	}
	var full map[string]any
	if err := json.Unmarshal(result.Raw, &full); err != nil {
		t.Fatal(err)
	}
	if len(full["data"].([]any)) != 3 || full["publication_time_status"] != "unknown" {
		t.Fatal("input history or clocks changed")
	}
}
func TestPointersNullAbsenceAndUnicode(t *testing.T) {
	raw := []byte(`{"a/b":{"~key":"kept"},"":9,"text":"\u00c5\ud83d\ude42","nullable":null,"empty":[]}`)
	for _, rule := range []string{`field('/a~1b/~0key') == 'kept'`, `field('/') == 9`, `length('/text') == 2`, `exists('/nullable') && !exists('/missing') && json('/nullable') == 'null'`, `length('/empty') == 0`, `length('') == 5`} {
		r, err := EvaluateJSON(raw, rule)
		if err != nil || r.Value != true {
			t.Fatal(rule, r, err)
		}
	}
	for _, rule := range []string{`field('/missing')`, `field('/a~2b')`, `field('not-a-pointer')`, `field('/empty/01')`, `field('/empty/-')`, `exists(1)`, `field()`} {
		if _, err := EvaluateJSON(raw, rule); err == nil {
			t.Fatal("invalid pointer/function accepted", rule)
		}
	}
	r, err := EvaluateJSON([]byte(`null`), `field('')`)
	if err != nil || r.Value != nil || string(r.Raw) != "null" {
		t.Fatal(r, err)
	}
}
func TestExactNumericTextAndNativeRange(t *testing.T) {
	raw := []byte(`{"id":9007199254740993,"decimal":1.2300,"huge":1e9999,"small":1.5,"string":"00123"}`)
	for _, rule := range []string{`json('/id') == '9007199254740993'`, `json('/decimal') == '1.2300'`, `json('/huge') == '1e9999'`, `field('/small') + 0.5 == 2`, `field('/string') == '00123'`} {
		r, err := EvaluateJSON(raw, rule)
		if err != nil || r.Value != true || !bytes.Equal(r.Raw, raw) {
			t.Fatal(rule, r, err)
		}
	}
	for _, rule := range []string{`field('/id')`, `field('/huge')`, `9007199254740993`, `1 / 0`} {
		if _, err := EvaluateJSON(raw, rule); err == nil {
			t.Fatal("unsafe native numeric accepted", rule)
		}
	}
}
func TestDateLiteralsDoNotConvertResponseClocks(t *testing.T) {
	raw := []byte(`{"date":"2026-11-01","release_time_status":"not_announced"}`)
	r, err := EvaluateJSONWithConstants(raw, `field('/date') == expected_date && field('/release_time_status') == 'not_announced'`, []byte(`{"expected_date":"2026-11-01"}`))
	if err != nil || r.Value != true {
		t.Fatal(r, err)
	}
	native, err := govaluate.NewEvaluableExpression(`'2014-01-02T12:00:00Z'`)
	if err != nil {
		t.Fatal(err)
	}
	timeToken := false
	for _, token := range native.Tokens() {
		if token.Kind == govaluate.TIME {
			timeToken = true
		}
	}
	if !timeToken {
		t.Fatal("native date behaviour changed; review the adapter boundary")
	}
	if _, err := EvaluateJSON(raw, `'2014-01-02T12:00:00Z'`); err == nil {
		t.Fatal("implicit native date accepted")
	}
}
func TestMalformedAndBoundedInputs(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte(`{"x":1,"x":2}`), []byte(`{} {}`), []byte(`{"x":`), {0xff}, []byte(strings.Repeat("[", MaxJSONDepth+2) + "0" + strings.Repeat("]", MaxJSONDepth+2)), []byte(`{"x":"` + strings.Repeat("x", MaxResponseBytes) + `"}`), []byte("[" + strings.Repeat("0,", MaxJSONValues) + "0]")} {
		if _, err := EvaluateJSON(raw, `true`); err == nil {
			t.Fatal("malformed or unbounded JSON accepted")
		}
	}
	for _, rule := range []string{"", strings.Repeat("x", MaxRuleBytes+1), strings.Repeat("field('/n') + ", 150) + "1", `unknown`, `os.Getenv('HOME')`, `field('') ==`} {
		if _, err := EvaluateJSON([]byte(`{"n":1}`), rule); err == nil {
			t.Fatal("invalid/unbounded rule accepted")
		}
	}
	if _, err := EvaluateJSON([]byte(`{"x":[1]}`), `field('/x')`); err == nil {
		t.Fatal("container silently converted to scalar")
	}
}
func TestMaterializedViewBudgetAndErrorRedaction(t *testing.T) {
	raw := []byte(`{"text":"` + strings.Repeat("x", MaxResponseBytes-100) + `"}`)
	if _, err := EvaluateJSON(raw, `field('/text') + field('/text') + field('/text')`); err == nil {
		t.Fatal("aggregate view budget not enforced")
	}
	_, err := EvaluateJSON([]byte(`{"private":"fictional-private-marker"}`), `field('/private') - 1`)
	if err == nil || strings.Contains(err.Error(), "fictional-private-marker") {
		t.Fatal("evaluation error leaked response", err)
	}
}

type typedFixture struct {
	Currency string `json:"currency"`
	Hidden   string `json:"-"`
}

func (typedFixture) DangerousMethod() string { panic("method must not be exposed") }
func TestTypedResponsesUseJSONAndNeverExposeMethods(t *testing.T) {
	r, err := Evaluate(typedFixture{Currency: "USD", Hidden: "fictional"}, `field('/currency') == 'USD' && !exists('/Hidden')`)
	if err != nil || r.Value != true {
		t.Fatal(r, err)
	}
	if _, err := Evaluate(make(chan int), `true`); err == nil {
		t.Fatal("non-JSON accepted")
	}
	if _, err := Evaluate(typedFixture{}, `DangerousMethod()`); err == nil {
		t.Fatal("Go method exposed")
	}
}
func TestNativeRulesForEveryJSONResponseFamily(t *testing.T) {
	fixtures := map[string]string{
		"catalogue":                              `{"currency":"USD","data":[],"coverage":{"future":null}}`,
		"announcements_revisions":                `{"data":[{"revision":0,"value":1},{"revision":1,"value":null}],"publication_time_status":"unknown"}`,
		"calendar":                               `{"data":[{"release_time_status":"not_announced","source":"fictional"}]}`,
		"predictions_coverage":                   `{"data":[],"coverage":{"available":false}}`,
		"central_bank":                           `{"data":[{"headline":"fictional fixture","provenance":{"official":false}}]}`,
		"forex":                                  `{"base":"USD","quote":"EUR","data":[{"rate":1.234500}]}`,
		"sources":                                `{"data":[],"provenance":{"source":"fictional offline fixture"}}`,
		"cot_commodities":                        `{"data":[{"net":null}],"future_quality":{"kept":true}}`,
		"sessions_prices":                        `{"data":[],"sessions":[],"prices":[]}`,
		"curves_factors_differentials_sentiment": `{"data":[],"curve":[],"factors":[],"sentiment":null}`,
		"research":                               `{"panels":[{"data":[],"model":"fictional"}],"request":{"as_of":"2026-11-01"}}`,
		"seasonality_news_future":                `{"future":{"unknown":[null,true,9007199254740993]},"data":[]}`,
		"array":                                  `[null,{"unchanged":true}]`, "scalar": `"unchanged"`, "boolean": `true`, "null": `null`,
	}
	for name, raw := range fixtures {
		t.Run(name, func(t *testing.T) {
			r, err := EvaluateJSON([]byte(raw), `exists('')`)
			if err != nil || r.Value != true || string(r.Raw) != raw {
				t.Fatal(r, err)
			}
		})
	}
}

// Availability of public SDK operations is separate from native JSON fixture
// coverage. These checks issue no request and do not assert live API coverage.
func TestAllPublicSDKOperationsRemainAvailable(t *testing.T) {
	names := []string{"GetAnnouncementChanges", "GetAnnouncements", "GetCalendar", "GetCommodities", "GetCot", "GetCurves", "GetDataCatalogue", "GetFactor", "GetFinancialPrices", "GetForex", "GetFxIntradayReferenceRates", "GetFxReferenceRates", "GetFxSessionRates", "GetFxSourceUniverse", "GetFxSources", "GetHealth", "GetLatestAnnouncements", "GetLatestCommodities", "GetMarketSessions", "GetPredictionCoverage", "GetPredictionCoverageForCurrency", "GetPredictions", "GetPressReleases", "GetRateDifferentials", "GetRiskSentiment", "Ping", "ResearchPanel"}
	api := reflect.TypeOf(sdk.NewAPIClient(sdk.NewConfiguration()).FXMacroDataAPI)
	for _, name := range names {
		if _, ok := api.MethodByName(name); !ok {
			t.Fatal("public SDK method unavailable", name)
		}
	}
}
func TestConcurrentNativeEvaluationsAreIsolated(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			raw := []byte(fmt.Sprintf(`{"n":%d}`, n))
			r, err := EvaluateJSON(raw, fmt.Sprintf(`field('/n') == %d`, n))
			if err != nil || r.Value != true || !bytes.Equal(r.Raw, raw) {
				t.Errorf("evaluation %d lost isolation: %v", n, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestExplicitJSONConstantsAreBoundedScalars(t *testing.T) {
	raw := []byte(`{"value":0,"flag":false,"text":"unchanged"}`)
	r, err := EvaluateJSONWithConstants(raw, `field('/value') == zero && field('/flag') == flag && field('/text') == text`, []byte(`{"zero":0,"flag":false,"text":"unchanged"}`))
	if err != nil || r.Value != true || !bytes.Equal(r.Raw, raw) {
		t.Fatal(r, err)
	}
	for _, constants := range []string{`null`, `[]`, `{"x":[]}`, `{"x":{}}`, `{"x":9007199254740993}`, `{"field":1}`, `{"bad name":1}`, `{"x":1,"x":2}`, `{} {}`, `{"x":"` + strings.Repeat("x", MaxConstantsBytes) + `"}`} {
		if _, err := EvaluateJSONWithConstants(raw, `true`, []byte(constants)); err == nil {
			t.Fatal("invalid constants accepted")
		}
	}
	if _, err := EvaluateJSONWithConstants(raw, `not_supplied`, []byte(`{"supplied":1}`)); err == nil {
		t.Fatal("unbound native variable accepted")
	}
}
