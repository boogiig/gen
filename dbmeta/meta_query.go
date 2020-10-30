package dbmeta

import (
	"database/sql"
	"fmt"
	"strings"
)

// Queries :
var Queries map[string]*QueryMapping

// LoadQueryMeta fetch db meta data for database
func LoadQueryMeta(db *sql.DB, sqlType, sqlDatabase string, queryMapping *QueryMapping) (DbTableMeta, error) {
	m := &dbTableMeta{
		sqlType:     sqlType,
		sqlDatabase: sqlDatabase,
		tableName:   queryMapping.QueryName,
	}

	colInfo, err := LoadQueryInfoFromInformationSchema(db, sqlDatabase, queryMapping)
	if err != nil {
		return nil, fmt.Errorf("unable to load identity info schema from postgres table: %s error: %v", queryMapping.QueryName, err)
	}

	m.columns = make([]*columnMeta, len(colInfo))

	ordinal := 0

	for _, v := range colInfo {
		defaultVal := ""
		nullable := true
		isAutoIncrement := false
		isPrimaryKey := v.ColumnName == "id"
		var maxLen int64

		maxLen = v.CharacterMaximumLength.(int64)
		definedType := v.DataType
		colDDL := v.DataType
		if definedType == "" {
			definedType = "USER_DEFINED"
			colDDL = "VARCHAR"
		}

		colMeta := &columnMeta{
			index:            ordinal,
			name:             v.ColumnName,
			databaseTypeName: colDDL,
			nullable:         nullable,
			isPrimaryKey:     isPrimaryKey,
			isAutoIncrement:  isAutoIncrement,
			colDDL:           colDDL,
			columnLen:        maxLen,
			columnType:       definedType,
			defaultVal:       defaultVal,
		}

		m.columns[ordinal] = colMeta
		ordinal = ordinal + 1
	}

	m.ddl = ""
	return m, nil
}

// LoadQueryInfoFromInformationSchema fetch info from information_schema for database
func LoadQueryInfoFromInformationSchema(db *sql.DB, sqlDatabase string, queryMapping *QueryMapping) (primaryKey map[string]*InformationSchema, err error) {
	colInfo := make(map[string]*InformationSchema)

	rows, err := db.Query(queryMapping.Query)
	defer rows.Close()
	types, _ := rows.ColumnTypes()
	ordinal := 0
	for _, t := range types {
		// fmt.Printf("%v\n", t)
		ordinal = ordinal + 1
		ci := &InformationSchema{}
		ci.TableCatalog = sqlDatabase
		ci.TableSchema = "public"
		ci.TableName = queryMapping.QueryName
		ci.ColumnName = strings.ToLower(t.Name())
		ci.OrdinalPosition = ordinal
		ci.DataType = strings.ToLower(t.DatabaseTypeName())
		if len, ok := t.Length(); ok {
			ci.CharacterMaximumLength = len
		} else {
			ci.CharacterMaximumLength = int64(0)
		}
		ci.ColumnDefault = nil
		ci.IsNullable = "YES"

		colInfo[ci.ColumnName] = ci
	}

	return colInfo, nil
}

// LoadQueryInfo load table info from db connection, and list of tables
func LoadQueryInfo(db *sql.DB, conf *Config) map[string]*ModelInfo {

	tableInfos := make(map[string]*ModelInfo)

	// generate go files for each table
	var tableIdx = 0
	for _, queryMapping := range Queries {

		// _, ok := FindInSlice(excludeDbTables, tableName)
		// if ok {
		// 	fmt.Printf("Skipping excluded table %s\n", tableName)
		// 	continue
		// }

		dbMeta, err := LoadQMeta(conf.SQLType, db, conf.SQLDatabase, queryMapping)
		if err != nil {
			msg := fmt.Sprintf("Warning - LoadMeta skipping table info for %s error: %v\n", queryMapping.QueryName, err)
			if au != nil {
				fmt.Print(au.Yellow(msg))
			} else {
				fmt.Printf(msg)
			}

			continue
		}

		modelInfo, err := GenerateModelInfo(tableInfos, dbMeta, queryMapping.QueryName, conf)
		if err != nil {
			msg := fmt.Sprintf("Error - %v\n", err)
			if au != nil {
				fmt.Print(au.Red(msg))
			} else {
				fmt.Printf(msg)
			}

			continue
		}

		if len(modelInfo.Fields) == 0 {
			if conf.Verbose {
				fmt.Printf("Table: %s - No Fields Available\n", queryMapping.QueryName)
			}
			continue
		}

		modelInfo.Index = tableIdx
		modelInfo.IndexPlus1 = tableIdx + 1
		modelInfo.IsQuery = true
		modelInfo.Query = queryMapping.Query
		tableIdx++

		tableInfos[queryMapping.QueryName] = modelInfo
	}

	return tableInfos
}
