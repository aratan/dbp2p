package db

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// QueryOperator define los operadores de consulta
type QueryOperator string

const (
	// OperatorEQ igual a
	OperatorEQ QueryOperator = "eq"
	// OperatorNE no igual a
	OperatorNE QueryOperator = "ne"
	// OperatorGT mayor que
	OperatorGT QueryOperator = "gt"
	// OperatorGTE mayor o igual que
	OperatorGTE QueryOperator = "gte"
	// OperatorLT menor que
	OperatorLT QueryOperator = "lt"
	// OperatorLTE menor o igual que
	OperatorLTE QueryOperator = "lte"
	// OperatorIN en
	OperatorIN QueryOperator = "in"
	// OperatorNIN no en
	OperatorNIN QueryOperator = "nin"
	// OperatorREGEX expresión regular
	OperatorREGEX QueryOperator = "regex"
	// OperatorEXISTS existe
	OperatorEXISTS QueryOperator = "exists"
	// OperatorTYPE tipo
	OperatorTYPE QueryOperator = "type"
	// OperatorCONTAINS contiene
	OperatorCONTAINS QueryOperator = "contains"
	// OperatorSTARTSWITH comienza con
	OperatorSTARTSWITH QueryOperator = "startswith"
	// OperatorENDSWITH termina con
	OperatorENDSWITH QueryOperator = "endswith"
)

// LogicalOperator define los operadores lógicos
type LogicalOperator string

const (
	// LogicalAND operador AND
	LogicalAND LogicalOperator = "and"
	// LogicalOR operador OR
	LogicalOR LogicalOperator = "or"
	// LogicalNOT operador NOT
	LogicalNOT LogicalOperator = "not"
)

// SortDirection define la dirección de ordenación
type SortDirection string

const (
	// SortAscending orden ascendente
	SortAscending SortDirection = "asc"
	// SortDescending orden descendente
	SortDescending SortDirection = "desc"
)

// QueryCondition representa una condición de consulta
type QueryCondition struct {
	Field    string        `json:"field"`
	Operator QueryOperator `json:"operator"`
	Value    interface{}   `json:"value"`
}

// LogicalCondition representa una condición lógica
type LogicalCondition struct {
	Operator   LogicalOperator `json:"operator"`
	Conditions []interface{}   `json:"conditions"` // Puede contener QueryCondition o LogicalCondition
}

// SortOption representa una opción de ordenación
type SortOption struct {
	Field     string        `json:"field"`
	Direction SortDirection `json:"direction"`
}

// QueryOptions representa las opciones de consulta
type QueryOptions struct {
	Skip  int          `json:"skip"`
	Limit int          `json:"limit"`
	Sort  []SortOption `json:"sort"`
}

// Query representa una consulta avanzada
type Query struct {
	Collection string       `json:"collection"`
	Condition  interface{}  `json:"condition"` // Puede ser QueryCondition o LogicalCondition
	Options    QueryOptions `json:"options"`
}

// NewQuery crea una nueva consulta
func NewQuery(collection string) *Query {
	return &Query{
		Collection: collection,
		Options: QueryOptions{
			Limit: 100, // Límite por defecto
		},
	}
}

// Where establece una condición simple
func (q *Query) Where(field string, operator QueryOperator, value interface{}) *Query {
	q.Condition = QueryCondition{
		Field:    field,
		Operator: operator,
		Value:    value,
	}
	return q
}

// And combina condiciones con AND
func (q *Query) And(conditions ...interface{}) *Query {
	q.Condition = LogicalCondition{
		Operator:   LogicalAND,
		Conditions: conditions,
	}
	return q
}

// Or combina condiciones con OR
func (q *Query) Or(conditions ...interface{}) *Query {
	q.Condition = LogicalCondition{
		Operator:   LogicalOR,
		Conditions: conditions,
	}
	return q
}

// Not niega una condición
func (q *Query) Not(condition interface{}) *Query {
	q.Condition = LogicalCondition{
		Operator:   LogicalNOT,
		Conditions: []interface{}{condition},
	}
	return q
}

// Skip establece el número de documentos a omitir
func (q *Query) Skip(skip int) *Query {
	q.Options.Skip = skip
	return q
}

// Limit establece el número máximo de documentos a devolver
func (q *Query) Limit(limit int) *Query {
	q.Options.Limit = limit
	return q
}

// Sort establece las opciones de ordenación
func (q *Query) Sort(field string, direction SortDirection) *Query {
	q.Options.Sort = append(q.Options.Sort, SortOption{
		Field:     field,
		Direction: direction,
	})
	return q
}

// Execute ejecuta la consulta
func (q *Query) Execute(db *Database) ([]*Document, error) {
	// Obtener todos los documentos de la colección
	docs, err := db.GetAllDocuments(q.Collection)
	if err != nil {
		return nil, err
	}

	// Filtrar documentos según la condición
	var results []*Document
	for _, doc := range docs {
		if q.matchesCondition(doc.Data, q.Condition) {
			results = append(results, doc)
		}
	}

	// Aplicar ordenación
	if len(q.Options.Sort) > 0 {
		results = q.sortDocuments(results)
	}

	// Aplicar paginación
	if q.Options.Skip > 0 && q.Options.Skip < len(results) {
		results = results[q.Options.Skip:]
	}
	if q.Options.Limit > 0 && q.Options.Limit < len(results) {
		results = results[:q.Options.Limit]
	}

	return results, nil
}

// matchesCondition verifica si un documento coincide con una condición
func (q *Query) matchesCondition(data map[string]interface{}, condition interface{}) bool {
	// Verificar tipo de condición
	switch cond := condition.(type) {
	case QueryCondition:
		return q.matchesQueryCondition(data, cond)
	case LogicalCondition:
		return q.matchesLogicalCondition(data, cond)
	case map[string]interface{}:
		// Convertir mapa a QueryCondition
		if field, ok := cond["field"].(string); ok {
			if operator, ok := cond["operator"].(string); ok {
				return q.matchesQueryCondition(data, QueryCondition{
					Field:    field,
					Operator: QueryOperator(operator),
					Value:    cond["value"],
				})
			}
		}
		// Verificar si es una condición lógica
		if operator, ok := cond["operator"].(string); ok {
			if conditions, ok := cond["conditions"].([]interface{}); ok {
				return q.matchesLogicalCondition(data, LogicalCondition{
					Operator:   LogicalOperator(operator),
					Conditions: conditions,
				})
			}
		}
	}
	return false
}

// matchesQueryCondition verifica si un documento coincide con una condición de consulta
func (q *Query) matchesQueryCondition(data map[string]interface{}, condition QueryCondition) bool {
	// Obtener valor del campo
	fieldValue, err := getNestedFieldValue(data, condition.Field)
	if err != nil {
		// Si el campo no existe, verificar si es una condición de existencia
		if condition.Operator == OperatorEXISTS {
			return condition.Value.(bool) == false
		}
		return false
	}

	// Si el campo existe y es una condición de existencia
	if condition.Operator == OperatorEXISTS {
		return condition.Value.(bool) == true
	}

	// Comparar según el operador
	switch condition.Operator {
	case OperatorEQ:
		return compareValues(fieldValue, condition.Value) == 0
	case OperatorNE:
		return compareValues(fieldValue, condition.Value) != 0
	case OperatorGT:
		return compareValues(fieldValue, condition.Value) > 0
	case OperatorGTE:
		return compareValues(fieldValue, condition.Value) >= 0
	case OperatorLT:
		return compareValues(fieldValue, condition.Value) < 0
	case OperatorLTE:
		return compareValues(fieldValue, condition.Value) <= 0
	case OperatorIN:
		// Verificar si el valor está en la lista
		if values, ok := condition.Value.([]interface{}); ok {
			for _, value := range values {
				if compareValues(fieldValue, value) == 0 {
					return true
				}
			}
		}
		return false
	case OperatorNIN:
		// Verificar si el valor no está en la lista
		if values, ok := condition.Value.([]interface{}); ok {
			for _, value := range values {
				if compareValues(fieldValue, value) == 0 {
					return false
				}
			}
		}
		return true
	case OperatorREGEX:
		// Verificar si el valor coincide con la expresión regular
		if strValue, ok := fieldValue.(string); ok {
			if pattern, ok := condition.Value.(string); ok {
				matched, err := regexp.MatchString(pattern, strValue)
				return err == nil && matched
			}
		}
		return false
	case OperatorTYPE:
		// Verificar si el valor es del tipo especificado
		if typeName, ok := condition.Value.(string); ok {
			return getTypeName(fieldValue) == typeName
		}
		return false
	case OperatorCONTAINS:
		// Verificar si el valor contiene el texto especificado
		if strValue, ok := fieldValue.(string); ok {
			if subStr, ok := condition.Value.(string); ok {
				return strings.Contains(strValue, subStr)
			}
		}
		return false
	case OperatorSTARTSWITH:
		// Verificar si el valor comienza con el texto especificado
		if strValue, ok := fieldValue.(string); ok {
			if prefix, ok := condition.Value.(string); ok {
				return strings.HasPrefix(strValue, prefix)
			}
		}
		return false
	case OperatorENDSWITH:
		// Verificar si el valor termina con el texto especificado
		if strValue, ok := fieldValue.(string); ok {
			if suffix, ok := condition.Value.(string); ok {
				return strings.HasSuffix(strValue, suffix)
			}
		}
		return false
	}

	return false
}

// matchesLogicalCondition verifica si un documento coincide con una condición lógica
func (q *Query) matchesLogicalCondition(data map[string]interface{}, condition LogicalCondition) bool {
	switch condition.Operator {
	case LogicalAND:
		// Todas las condiciones deben cumplirse
		for _, cond := range condition.Conditions {
			if !q.matchesCondition(data, cond) {
				return false
			}
		}
		return true
	case LogicalOR:
		// Al menos una condición debe cumplirse
		for _, cond := range condition.Conditions {
			if q.matchesCondition(data, cond) {
				return true
			}
		}
		return false
	case LogicalNOT:
		// La condición no debe cumplirse
		if len(condition.Conditions) > 0 {
			return !q.matchesCondition(data, condition.Conditions[0])
		}
	}
	return false
}

// sortDocuments ordena los documentos según las opciones de ordenación
func (q *Query) sortDocuments(docs []*Document) []*Document {
	// Implementar ordenación
	// Por simplicidad, solo se implementa la ordenación por un campo
	if len(q.Options.Sort) > 0 {
		sortOption := q.Options.Sort[0]
		field := sortOption.Field
		direction := sortOption.Direction

		// Ordenar documentos
		for i := 0; i < len(docs)-1; i++ {
			for j := i + 1; j < len(docs); j++ {
				// Obtener valores de los campos
				valueI, errI := getNestedFieldValue(docs[i].Data, field)
				valueJ, errJ := getNestedFieldValue(docs[j].Data, field)

				// Si algún campo no existe, moverlo al final
				if errI != nil && errJ == nil {
					docs[i], docs[j] = docs[j], docs[i]
					continue
				}
				if errI == nil && errJ != nil {
					continue
				}
				if errI != nil && errJ != nil {
					continue
				}

				// Comparar valores
				cmp := compareValues(valueI, valueJ)
				if (direction == SortAscending && cmp > 0) || (direction == SortDescending && cmp < 0) {
					docs[i], docs[j] = docs[j], docs[i]
				}
			}
		}
	}

	return docs
}

// getNestedFieldValue obtiene el valor de un campo, soportando notación de punto para campos anidados
func getNestedFieldValue(data map[string]interface{}, field string) (interface{}, error) {
	// Verificar si el campo contiene notación de punto
	parts := strings.Split(field, ".")
	if len(parts) == 1 {
		// Campo simple
		value, exists := data[field]
		if !exists {
			return nil, fmt.Errorf("campo no encontrado: %s", field)
		}
		return value, nil
	}

	// Campo anidado
	current := data
	for i, part := range parts {
		if i == len(parts)-1 {
			// Última parte
			value, exists := current[part]
			if !exists {
				return nil, fmt.Errorf("campo no encontrado: %s", field)
			}
			return value, nil
		}

		// Parte intermedia
		next, exists := current[part]
		if !exists {
			return nil, fmt.Errorf("campo no encontrado: %s", field)
		}

		// Verificar si es un mapa
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("campo no es un objeto: %s", part)
		}

		current = nextMap
	}

	return nil, fmt.Errorf("campo no encontrado: %s", field)
}

// compareValues compara dos valores
func compareValues(a, b interface{}) int {
	// Si los tipos son diferentes, convertir a string
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		aStr := fmt.Sprintf("%v", a)
		bStr := fmt.Sprintf("%v", b)
		return strings.Compare(aStr, bStr)
	}

	// Comparar según el tipo
	switch a := a.(type) {
	case string:
		return strings.Compare(a, b.(string))
	case int:
		if a < b.(int) {
			return -1
		} else if a > b.(int) {
			return 1
		}
		return 0
	case int64:
		if a < b.(int64) {
			return -1
		} else if a > b.(int64) {
			return 1
		}
		return 0
	case float64:
		if a < b.(float64) {
			return -1
		} else if a > b.(float64) {
			return 1
		}
		return 0
	case bool:
		if !a && b.(bool) {
			return -1
		} else if a && !b.(bool) {
			return 1
		}
		return 0
	case time.Time:
		if a.Before(b.(time.Time)) {
			return -1
		} else if a.After(b.(time.Time)) {
			return 1
		}
		return 0
	}

	// Para otros tipos, convertir a string
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.Compare(aStr, bStr)
}

// getTypeName obtiene el nombre del tipo de un valor
func getTypeName(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "float"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case time.Time:
		return "date"
	default:
		return reflect.TypeOf(value).String()
	}
}
