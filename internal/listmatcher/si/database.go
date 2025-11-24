package si

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"openPAQ/internal/listmatcher/types"
	"openPAQ/internal/normalization"
)

type Database interface {
	GetPostalCodeStreet(normalizer normalization.Normalize) (map[string]PostalCodeStreetItems, error)
	GetCityPostalCode(normalizer normalization.Normalize) (map[string]CityPostalCodeItems, error)
}

type ClickhouseDatabase struct {
	db       clickhouse.Conn
	database string
	table    string
}

type LookupList struct {
	Name       string `ch:"ulica_naziv"`
	PostalCode string `ch:"postni_okolis_sifra"`
	City       string `ch:"naselje_naziv"`
}

func NewDatabase(config types.DatabaseConfig) *ClickhouseDatabase {
	return &ClickhouseDatabase{
		db:       initClickhouseConfig(config),
		database: config.DataBase,
		table:    config.Table,
	}
}

func initClickhouseConfig(config types.DatabaseConfig) (db clickhouse.Conn) {
	db, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", config.DbHost, config.DbPort)},
		Auth: clickhouse.Auth{
			Database: config.DataBase,
			Username: config.DbUserName,
			Password: config.DbUserPassword,
		},
	})
	if err != nil {
		panic(fmt.Sprintf("error connecting to database: %s", err))
	}
	return db
}

func (db *ClickhouseDatabase) GetPostalCodeStreet(normalizer normalization.Normalize) (map[string]PostalCodeStreetItems, error) {

	result := make(map[string]PostalCodeStreetItems)

	type LookupList struct {
		PostalCode string `ch:"postni_okolis_sifra"`
		Street     string `ch:"ulica_naziv"`
	}

	var dbResponse []LookupList

	// Include both addresses with street names AND addresses without street names (using settlement name)
	// For Slovenian addresses, when ulica_naziv is empty, the settlement name (naselje_naziv) acts as the "street"
	query := `
		SELECT DISTINCT postni_okolis_sifra, 
			CASE 
				WHEN ulica_naziv != '' THEN ulica_naziv
				ELSE naselje_naziv
			END AS ulica_naziv
		FROM {database:Identifier}.{table:Identifier} 
		WHERE postni_okolis_sifra != '' AND (ulica_naziv != '' OR naselje_naziv != '')
		ORDER BY postni_okolis_sifra
	`

	ctx := clickhouse.Context(context.Background(), clickhouse.WithParameters(clickhouse.Parameters{
		"database": db.database,
		"table":    db.table,
	}))

	err := db.db.Select(ctx, &dbResponse, query)

	if err != nil {
		return nil, err
	}

	for _, entry := range dbResponse {

		normalizedPostalCode, err := normalizer.PostalCode(entry.PostalCode)
		if err != nil {
			continue
		}

		normalizedStreets, err := normalizer.Street(entry.Street)
		if err != nil {
			continue
		}

		postalCodeStreetItem, ok := result[entry.PostalCode]

		var streets []NormalizeEnclosure

		for _, normalizedStreet := range normalizedStreets {
			streets = append(streets, NormalizeEnclosure{
				Raw:        entry.Street,
				Normalized: normalizedStreet,
			})
		}

		if ok {
			streets = append(streets, postalCodeStreetItem.Streets...)
		}

		result[normalizedPostalCode] = PostalCodeStreetItems{
			PostalCode: entry.PostalCode,
			Streets:    streets,
		}

	}

	return result, nil
}

func (db *ClickhouseDatabase) GetCityPostalCode(normalizer normalization.Normalize) (map[string]CityPostalCodeItems, error) {

	result := make(map[string]CityPostalCodeItems)

	type LookupList struct {
		City       string `ch:"naselje_naziv"`
		PostalCode string `ch:"postni_okolis_sifra"`
	}

	var dbResponse []LookupList

	query := `
		SELECT DISTINCT naselje_naziv, postni_okolis_sifra
		FROM {database:Identifier}.{table:Identifier} 
		WHERE naselje_naziv != '' AND postni_okolis_sifra != ''
		ORDER BY naselje_naziv
	`

	ctx := clickhouse.Context(context.Background(), clickhouse.WithParameters(clickhouse.Parameters{
		"database": db.database,
		"table":    db.table,
	}))

	err := db.db.Select(ctx, &dbResponse, query)

	if err != nil {
		return nil, err
	}

	for _, entry := range dbResponse {

		normalizedPostalCode, err := normalizer.PostalCode(entry.PostalCode)
		if err != nil || normalizedPostalCode == "" {
			continue
		}

		postalCodeItem := NormalizeEnclosure{
			Raw:        entry.PostalCode,
			Normalized: normalizedPostalCode,
		}

		normalizedCity, err := normalizer.City(entry.City)
		if err != nil {
			continue
		}

		cityPostalCodeItem, ok := result[normalizedCity]

		if !ok {
			result[normalizedCity] = CityPostalCodeItems{
				City: entry.City,
				PostalCodes: []NormalizeEnclosure{
					postalCodeItem,
				},
			}
		} else {
			temp := cityPostalCodeItem.PostalCodes
			found := false
			for _, v := range temp {
				if v.Normalized == normalizedPostalCode {
					found = true
				}
			}

			if !found {
				temp = append(temp, postalCodeItem)

				result[normalizedCity] = CityPostalCodeItems{
					City:        result[normalizedCity].City,
					PostalCodes: temp,
				}
			}

		}

	}

	return result, nil
}

