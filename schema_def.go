package goparquet

import (
	"bytes"
	"fmt"
	"io"

	"github.com/fraugster/parquet-go/parquet"
)

// SchemaDefinition represents a valid textual schema definition.
type SchemaDefinition struct {
	col *column
}

// ParseSchemaDefinition parses a textual schema definition and returns
// an object, or an error if parsing has failed. The textual schema definition
// needs to adhere to the following grammar:
//
//	message ::= 'message' <identifier> '{' <message-body> '}'
//	message-body ::= <column-definition>*
//	column-definition ::= <repetition-type> <column-type-definition>
//	repetition-type ::= 'required' | 'repeated' | 'optional'
//	column-type-definition ::= <group-definition> | <field-definition>
//	group-definition ::= 'group' <identifier> <converted-type-annotation>? '{' <message-body> '}'
//	field-definition ::= <type> <identifier> <logical-type-annotation>? <field-id-definition>? ';'
//	type ::= 'binary'
//		| 'float'
//		| 'double'
//		| 'boolean'
//		| 'int32'
//		| 'int64'
//		| 'int96'
//		| 'fixed_len_byte_array' '(' <number> ')'
//	converted-type-annotation ::= '(' <converted-type> ')'
//	converted-type ::= 'UTF8'
//		| 'MAP'
//		| 'MAP_KEY_VALUE'
//		| 'LIST'
//		| 'ENUM'
//		| 'DECIMAL'
//		| 'DATE'
//		| 'TIME_MILLIS'
//		| 'TIME_MICROS'
//		| 'TIMESTAMP_MILLIS'
//		| 'TIMESTAMP_MICROS'
//		| 'UINT_8'
//		| 'UINT_16'
//		| 'UINT_32'
//		| 'UINT_64'
//		| 'INT_8'
//		| 'INT_16'
//		| 'INT_32'
//		| 'INT_64'
//		| 'JSON'
//		| 'BSON'
//		| 'INTERVAL'
//	logical-type-annotation ::= '(' <logical-type> ')'
//	logical-type ::= 'STRING'
//		| 'DATE'
//		| 'TIMESTAMP' '(' <time-unit> ',' <boolean> ')'
//		| 'UUID'
//		| 'ENUM'
//		| 'JSON'
//	field-id-definition ::= '=' <number>
//	number ::= <digit>+
//	digit ::= '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9'
//	time-unit ::= 'MILLIS' | 'MICROS' | 'NANOS'
//	boolean ::= 'false' | 'true'
//	identifier ::= <alpha> <alphanum>*
//	alpha ::= 'a'..'z' | 'A'..'Z'
//	alphanum ::= <alpha> | <digit>
func ParseSchemaDefinition(schemaText string) (*SchemaDefinition, error) {
	p := newSchemaParser(schemaText)
	if err := p.parse(); err != nil {
		return nil, err
	}

	return &SchemaDefinition{
		col: p.root,
	}, nil
}

func (sd *SchemaDefinition) String() string {
	if sd.col == nil {
		return "message empty {\n}\n"
	}

	buf := new(bytes.Buffer)

	fmt.Fprintf(buf, "message %s {\n", sd.col.Name())

	printCols(buf, sd.col.children, 2)

	fmt.Fprintf(buf, "}\n")

	return buf.String()
}

// SubSchema returns the direct child of the current schema definition
// that matches the provided name. If no such child exists, nil is
// returned.
func (sd *SchemaDefinition) SubSchema(name string) *SchemaDefinition {
	for _, c := range sd.col.children {
		if c.name == name {
			return &SchemaDefinition{
				col: c,
			}
		}
	}
	return nil
}

// SchemaElement returns the schema element associated with the current
// schema definition. If no schema element is present, then nil is returned.
func (sd *SchemaDefinition) SchemaElement() *parquet.SchemaElement {
	if sd == nil {
		return nil
	}

	return sd.col.element
}

func printCols(w io.Writer, cols []*column, indent int) {
	for _, col := range cols {
		printIndent(w, indent)

		elem := col.Element()

		switch elem.GetRepetitionType() {
		case parquet.FieldRepetitionType_REPEATED:
			fmt.Fprintf(w, "repeated")
		case parquet.FieldRepetitionType_OPTIONAL:
			fmt.Fprintf(w, "optional")
		case parquet.FieldRepetitionType_REQUIRED:
			fmt.Fprintf(w, "required")
		}
		fmt.Fprintf(w, " ")

		if elem.Type == nil {
			fmt.Fprintf(w, "group %s", elem.GetName())
			if elem.ConvertedType != nil {
				fmt.Fprintf(w, " (%s)", getSchemaConvertedType(elem.GetConvertedType()))
			}
			fmt.Fprintf(w, " {\n")
			printCols(w, col.children, indent+2)

			printIndent(w, indent)
			fmt.Fprintf(w, "}\n")
		} else {
			typ := getSchemaType(elem)
			fmt.Fprintf(w, "%s %s", typ, elem.GetName())
			if elem.LogicalType != nil {
				fmt.Fprintf(w, " (%s)", getSchemaLogicalType(elem.GetLogicalType()))
			}
			if elem.FieldID != nil {
				fmt.Fprintf(w, " = %d", elem.GetFieldID())
			}
			fmt.Fprintf(w, ";\n")
		}
	}
}

func printIndent(w io.Writer, indent int) {
	for i := 0; i < indent; i++ {
		fmt.Fprintf(w, " ")
	}
}

func getSchemaType(elem *parquet.SchemaElement) string {
	switch elem.GetType() {

	case parquet.Type_BYTE_ARRAY:
		return "binary"
	case parquet.Type_FLOAT:
		return "float"
	case parquet.Type_DOUBLE:
		return "double"
	case parquet.Type_BOOLEAN:
		return "boolean"
	case parquet.Type_INT32:
		return "int32"
	case parquet.Type_INT64:
		return "int64"
	case parquet.Type_INT96:
		return "int96"
	case parquet.Type_FIXED_LEN_BYTE_ARRAY:
		return fmt.Sprintf("fixed_len_byte_array(%d)", elem.GetTypeLength())
	}
	return fmt.Sprintf("UT:%s", elem.GetType())
}

func getSchemaConvertedType(t parquet.ConvertedType) string {
	switch t {
	case parquet.ConvertedType_UTF8:
		return "UTF8"
	case parquet.ConvertedType_LIST:
		return "LIST"
	case parquet.ConvertedType_MAP:
		return "MAP"
	case parquet.ConvertedType_MAP_KEY_VALUE:
		return "MAP_KEY_VALUE"
	}
	return fmt.Sprintf("UC:%s", t)
}

func getSchemaLogicalType(t *parquet.LogicalType) string {
	switch {
	case t.IsSetSTRING():
		return "STRING"
	case t.IsSetDATE():
		return "DATE"
	case t.IsSetTIMESTAMP():
		unit := ""
		switch {
		case t.TIMESTAMP.Unit.IsSetNANOS():
			unit = "NANOS"
		case t.TIMESTAMP.Unit.IsSetMICROS():
			unit = "MICROS"
		case t.TIMESTAMP.Unit.IsSetMILLIS():
			unit = "MILLIS"
		default:
			unit = "BUG_UNKNOWN_TIMESTAMP_UNIT"
		}
		return fmt.Sprintf("TIMESTAMP(%s, %t)", unit, t.TIMESTAMP.IsAdjustedToUTC)
	case t.IsSetUUID():
		return "UUID"
	case t.IsSetENUM():
		return "ENUM"
	case t.IsSetJSON():
		return "JSON"
	default:
		return "BUG(UNKNOWN)"
	}
}