package searchads360

import (
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

// CustomerWorkflow discovers customers directly accessible to the OAuth
// principal. The configured customer is used by the other workflows.
type CustomerWorkflow interface {
	ListAccessibleCustomers(context.Context, ...socialhub.CallOption) ([]string, error)
}

// ReportWorkflow executes bounded, paginated Search Ads 360 Query Language
// requests. SearchStream is gRPC-only and is not part of this REST adapter.
type ReportWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (SearchPage, error)
}

// CustomColumnWorkflow reads customer-owned Custom Column metadata.
type CustomColumnWorkflow interface {
	ListCustomColumns(context.Context, ...socialhub.CallOption) ([]CustomColumn, error)
	GetCustomColumn(context.Context, string, ...socialhub.CallOption) (*CustomColumn, error)
}

// FieldWorkflow discovers fields and compatibility for Search Ads 360 Query
// Language requests.
type FieldWorkflow interface {
	SearchFields(context.Context, FieldSearchRequest, ...socialhub.CallOption) (FieldPage, error)
	GetField(context.Context, string, ...socialhub.CallOption) (*Field, error)
}

type SummaryRowSetting string

const (
	SummaryRowUnspecified SummaryRowSetting = "UNSPECIFIED"
	SummaryRowUnknown     SummaryRowSetting = "UNKNOWN"
	SummaryRowNone        SummaryRowSetting = "NO_SUMMARY_ROW"
	SummaryRowWithResults SummaryRowSetting = "SUMMARY_ROW_WITH_RESULTS"
	SummaryRowOnly        SummaryRowSetting = "SUMMARY_ROW_ONLY"
)

type SearchRequest struct {
	Query                   string
	PageSize                int
	PageToken               string
	ValidateOnly            bool
	ReturnTotalResultsCount bool
	SummaryRowSetting       SummaryRowSetting
}

// Row preserves each requested top-level resource, metrics, segments, and
// custom-column values as exact JSON. This avoids float64 conversion and data
// loss as the large SearchAds360Row schema evolves.
type Row map[string]json.RawMessage

// Field returns a defensive copy of one top-level row field.
func (row Row) Field(name string) (json.RawMessage, bool) {
	value, found := row[name]
	if !found {
		return nil, false
	}
	return append(json.RawMessage(nil), value...), true
}

// DecodeField decodes one requested top-level field into a caller-owned type.
func (row Row) DecodeField(name string, output any) error {
	value, found := row[name]
	if !found {
		return fmt.Errorf("searchads360: row field %q is absent", name)
	}
	if output == nil {
		return fmt.Errorf("searchads360: row field destination is required")
	}
	return json.Unmarshal(value, output)
}

type ResultHeader struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CustomColumnHeader struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ReferencesMetrics bool   `json:"referencesMetrics"`
}

type SearchPage struct {
	Rows                               []Row
	SummaryRow                         Row
	NextPageToken                      string
	FieldMask                          string
	TotalResultsCount                  string
	ConversionCustomMetricHeaders      []ResultHeader
	ConversionCustomDimensionHeaders   []ResultHeader
	RawEventConversionMetricHeaders    []ResultHeader
	RawEventConversionDimensionHeaders []ResultHeader
	CustomColumnHeaders                []CustomColumnHeader
}

type searchResponse struct {
	Results                            []Row                `json:"results"`
	SummaryRow                         Row                  `json:"summaryRow"`
	NextPageToken                      string               `json:"nextPageToken"`
	FieldMask                          string               `json:"fieldMask"`
	TotalResultsCount                  string               `json:"totalResultsCount"`
	ConversionCustomMetricHeaders      []ResultHeader       `json:"conversionCustomMetricHeaders"`
	ConversionCustomDimensionHeaders   []ResultHeader       `json:"conversionCustomDimensionHeaders"`
	RawEventConversionMetricHeaders    []ResultHeader       `json:"rawEventConversionMetricHeaders"`
	RawEventConversionDimensionHeaders []ResultHeader       `json:"rawEventConversionDimensionHeaders"`
	CustomColumnHeaders                []CustomColumnHeader `json:"customColumnHeaders"`
}

type RenderType string

const (
	RenderUnspecified RenderType = "UNSPECIFIED"
	RenderUnknown     RenderType = "UNKNOWN"
	RenderNumber      RenderType = "NUMBER"
	RenderPercent     RenderType = "PERCENT"
	RenderMoney       RenderType = "MONEY"
	RenderString      RenderType = "STRING"
	RenderBoolean     RenderType = "BOOLEAN"
	RenderDate        RenderType = "DATE"
)

type ValueType string

const (
	ValueUnspecified ValueType = "UNSPECIFIED"
	ValueUnknown     ValueType = "UNKNOWN"
	ValueString      ValueType = "STRING"
	ValueInt64       ValueType = "INT64"
	ValueDouble      ValueType = "DOUBLE"
	ValueBoolean     ValueType = "BOOLEAN"
	ValueDate        ValueType = "DATE"
)

type CustomColumn struct {
	ResourceName            string          `json:"resourceName"`
	ID                      string          `json:"id"`
	Name                    string          `json:"name"`
	Description             string          `json:"description"`
	Queryable               bool            `json:"queryable"`
	RenderType              RenderType      `json:"renderType"`
	ValueType               ValueType       `json:"valueType"`
	ReferencesAttributes    bool            `json:"referencesAttributes"`
	ReferencesMetrics       bool            `json:"referencesMetrics"`
	ReferencedSystemColumns []string        `json:"referencedSystemColumns"`
	Raw                     json.RawMessage `json:"-"`
}

func (column *CustomColumn) UnmarshalJSON(data []byte) error {
	type alias CustomColumn
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*column = CustomColumn(decoded)
	column.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type FieldCategory string

const (
	FieldCategoryUnspecified FieldCategory = "UNSPECIFIED"
	FieldCategoryUnknown     FieldCategory = "UNKNOWN"
	FieldResource            FieldCategory = "RESOURCE"
	FieldAttribute           FieldCategory = "ATTRIBUTE"
	FieldSegment             FieldCategory = "SEGMENT"
	FieldMetric              FieldCategory = "METRIC"
)

type FieldDataType string

const (
	DataTypeUnspecified  FieldDataType = "UNSPECIFIED"
	DataTypeUnknown      FieldDataType = "UNKNOWN"
	DataTypeBoolean      FieldDataType = "BOOLEAN"
	DataTypeDate         FieldDataType = "DATE"
	DataTypeDouble       FieldDataType = "DOUBLE"
	DataTypeEnum         FieldDataType = "ENUM"
	DataTypeFloat        FieldDataType = "FLOAT"
	DataTypeInt32        FieldDataType = "INT32"
	DataTypeInt64        FieldDataType = "INT64"
	DataTypeMessage      FieldDataType = "MESSAGE"
	DataTypeResourceName FieldDataType = "RESOURCE_NAME"
	DataTypeString       FieldDataType = "STRING"
	DataTypeUint64       FieldDataType = "UINT64"
)

type Field struct {
	ResourceName       string          `json:"resourceName"`
	Name               string          `json:"name"`
	Category           FieldCategory   `json:"category"`
	DataType           FieldDataType   `json:"dataType"`
	TypeURL            string          `json:"typeUrl"`
	Selectable         bool            `json:"selectable"`
	Filterable         bool            `json:"filterable"`
	Sortable           bool            `json:"sortable"`
	Repeated           bool            `json:"isRepeated"`
	EnumValues         []string        `json:"enumValues"`
	SelectableWith     []string        `json:"selectableWith"`
	AttributeResources []string        `json:"attributeResources"`
	Metrics            []string        `json:"metrics"`
	Segments           []string        `json:"segments"`
	Raw                json.RawMessage `json:"-"`
}

func (field *Field) UnmarshalJSON(data []byte) error {
	type alias Field
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*field = Field(decoded)
	field.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type FieldSearchRequest struct {
	Query     string
	PageSize  int
	PageToken string
}

type FieldPage struct {
	Items             []Field
	NextPageToken     string
	TotalResultsCount string
}

type fieldSearchResponse struct {
	Results           []Field `json:"results"`
	NextPageToken     string  `json:"nextPageToken"`
	TotalResultsCount string  `json:"totalResultsCount"`
}
