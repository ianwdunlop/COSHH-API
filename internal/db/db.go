/*
 * Copyright 2024 Medicines Discovery Catapult
 * Licensed under the Apache License, Version 2.0 (the "Licence");
 * you may not use this file except in compliance with the Licence.
 * You may obtain a copy of the Licence at
 *     http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the Licence is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the Licence for the specific language governing permissions and
 * limitations under the Licence.
 */

package db

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/sethvargo/go-envconfig"
	"gitlab.mdcatapult.io/informatics/coshh/coshh-api/internal/chemical"
)

var db *sqlx.DB

var config Config

type Config struct {
	Port     int    `env:"PORT,default=5432"`
	User     string `env:"DBUSER,default=postgres"`
	Password string `env:"PASSWORD,default=postgres"`
	DbName   string `env:"DBNAME,default=informatics"`
	Host     string `env:"HOST,default=localhost"`
	Schema   string `env:"SCHEMA,default=coshh"`
	Retries  int    `env:"RETRIES,default=3"`
	SSLMode  string `env:"SSLMODE,default=require"`
}

func Connect() error {

	ctx := context.Background()

	if err := envconfig.Process(ctx, &config); err != nil {
		log.Println(" DB connect env vars unset or incorrect, using default config")
	}
	log.Printf("DB using env vars host=%s port=%d user=%s dbname=%s schema=%s retries=%d sslmode=%s\n", config.Host, config.Port, config.User, config.DbName, config.Schema, config.Retries, config.SSLMode)

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+"password=%s dbname=%s sslmode=%s", config.Host, config.Port, config.User, config.Password, config.DbName, config.SSLMode)

	var err error
	for i := 1; i < config.Retries; i++ {
		db, err = sqlx.Connect("postgres", psqlInfo)
		if err == nil {
			_, err = db.Exec(fmt.Sprintf("set search_path=%s", config.Schema))
			if err != nil {
				log.Printf("Failed to set search path to schema: %s\n", config.Schema)
				return err
			}
			log.Printf("Connected to database: %s, schema: %s\n", config.DbName, config.Schema)

			break
		}

		fmt.Println(err)
		fmt.Println("Failed to connect to DB, retrying in 5 seconds...")
		time.Sleep(5 * time.Second)
	}

	return err
}

func SelectAllChemicals() ([]chemical.Chemical, error) {
	chemicals := make([]chemical.Chemical, 0)
	query := fmt.Sprintf(`
		SELECT 
			c.id,
			c.cas_number,
			c.chemical_name,
			c.chemical_number,
			c.matter_state,
			c.quantity,
			c.added,
			c.expiry,
			c.safety_data_sheet,
			c.coshh_link,
			c.lab_location,
		    c.cupboard,
			c.storage_temp,
			c.is_archived,
			c.chemical_owner,
			string_agg(CAST(c2h.hazard AS VARCHAR(255)), ',') AS hazards 
		FROM %s.chemical c
		LEFT JOIN %s.chemical_to_hazard c2h ON c.id = c2h.id
		GROUP BY c.id`,
		config.Schema,
		config.Schema,
	)

	if err := db.Select(&chemicals, query); err != nil {
		return nil, err
	}

	for i := range chemicals {
		if chemicals[i].DBHazards != nil {
			chemicals[i].Hazards = strings.Split(*chemicals[i].DBHazards, ",")
		}
	}

	return chemicals, nil
}

func SelectAllCupboards() ([]string, error) {
	returnValue := make([]string, 0)

	query := fmt.Sprintf(`
		SELECT
		    DISTINCT COALESCE(c.cupboard, '')
		FROM %s.chemical c
	`, config.Schema,
	)

	if err := db.Select(&returnValue, query); err != nil {
		return nil, err
	}

	sort.Strings(returnValue)
	return returnValue, nil
}

/*
*
Given a lab location generate sql query to return all the cupboards for it
*/
func GetCupboardsForLab(lab string) ([]string, error) {
	returnValue := make([]string, 0)

	query := fmt.Sprintf("select DISTINCT cupboard from %s.chemical WHERE lab_location='%s'", config.Schema, lab)

	if err := db.Select(&returnValue, query); err != nil {
		return nil, err
	}

	sort.Strings(returnValue)
	return returnValue, nil
}

func UpdateChemical(chemical chemical.Chemical) error {
	query := fmt.Sprintf(`
	UPDATE %s.chemical
	SET 
		cas_number = :cas_number,
		chemical_name = :chemical_name,
		chemical_number = :chemical_number,
		matter_state = :matter_state,
		quantity = :quantity,
		added = :added,
		expiry = :expiry,
		safety_data_sheet = :safety_data_sheet,
		coshh_link = :coshh_link,
		lab_location = :lab_location,
	    cupboard = :cupboard,
		storage_temp = :storage_temp,
		is_archived = :is_archived,
		chemical_owner = :chemical_owner,
		last_updated_by = :last_updated_by
	WHERE id = :id
`, config.Schema,
	)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.NamedExec(query, chemical)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Fatalf("update failed: %v, unable to back: %v", err, rollbackErr)
		}

		return err
	}

	return err
}

func InsertChemical(chemical chemical.Chemical) (id int64, err error) {

	tx, err := db.Beginx()
	if err != nil {
		return 0, err
	}

	id, err = insertChemical(tx, chemical)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Fatalf("update failed: %v, unable to back: %v", err, rollbackErr)
		}

		return 0, err
	}

	if err := insertHazards(tx, chemical, id); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Fatalf("update failed: %v, unable to back: %v", err, rollbackErr)
		}

		return 0, err
	}

	return id, tx.Commit()
}

func insertChemical(tx *sqlx.Tx, chemical chemical.Chemical) (id int64, err error) {
	fmt.Printf("Chem Schema=%s", config.Schema)
	query := fmt.Sprintf(`INSERT INTO %s.chemical (
		cas_number,
		chemical_name,
		chemical_number,
		matter_state,
		quantity,
		added,
		expiry,
		safety_data_sheet,
		coshh_link,
		lab_location,
        cupboard,
		storage_temp,
		is_archived,
		chemical_owner
	)VALUES (
		:cas_number,
		:chemical_name,
		:chemical_number,
		:matter_state,
		:quantity,
		:added,
		:expiry,
		:safety_data_sheet,
		:coshh_link,
		:lab_location,
		:cupboard,
		:storage_temp,
		:is_archived,
		:chemical_owner
	) RETURNING id`,
		config.Schema,
	)

	rows, err := tx.NamedQuery(query, chemical)
	if err != nil {
		return
	}

	rows.Next()
	if err := rows.Scan(&id); err != nil {
		return 0, err
	}

	if err := rows.Close(); err != nil {
		return 0, err
	}

	return

}

func DeleteHazards(chemical chemical.Chemical) error {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`DELETE FROM %s.chemical_to_hazard WHERE id = $1;`, config.Schema) // DO NOT allow user input in raw SQL
	_, err = tx.ExecContext(ctx, query, chemical.Id)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Fatalf("update failed: %v, unable to back: %v", err, rollbackErr)
		}

		return err
	}

	return tx.Commit()
}

func InsertHazards(chemical chemical.Chemical) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}

	err = insertHazards(tx, chemical, chemical.Id)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Fatalf("update failed: %v, unable to back: %v", err, rollbackErr)
		}

		return err
	}

	return tx.Commit()
}

func insertHazards(tx *sqlx.Tx, chemical chemical.Chemical, id int64) error {
	// chemicalToHazard represents a row in chemical_to_hazard
	type chemicalToHazard struct {
		Id     int64  `db:"id"`
		Hazard string `db:"hazard"`
	}

	chemicalToHazards := make([]chemicalToHazard, 0)

	query := fmt.Sprintf(`INSERT INTO %s.chemical_to_hazard (id, hazard) VALUES (:id, :hazard)`, config.Schema) // DO NOT allow user input in raw SQL

	for _, hazard := range chemical.Hazards {
		chemicalToHazards = append(chemicalToHazards, chemicalToHazard{
			Id:     id,
			Hazard: hazard,
		})
	}

	_, err := tx.NamedExec(query, chemicalToHazards)
	return err
}

func GetMaxChemicalNumber() (string, error) {
	var returnValue string

	query := fmt.Sprintf(`SELECT COALESCE(MAX(chemical_number), '00000') FROM %s.chemical`, config.Schema)

	if err := db.Get(&returnValue, query); err != nil {
		return "", err
	}

	return returnValue, nil
}
