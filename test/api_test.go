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

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.mdcatapult.io/informatics/coshh/coshh-api/internal/chemical"
	"gitlab.mdcatapult.io/informatics/coshh/coshh-api/internal/db"
	"gitlab.mdcatapult.io/informatics/coshh/coshh-api/internal/server"
)

var cupboardsChem = chemical.Chemical{
	CasNumber:       stringPtr("12345678"),
	Name:            "beans",
	ChemicalNumber:  stringPtr("blueberries"),
	MatterState:     stringPtr("liquid"),
	Quantity:        stringPtr("5"),
	Cupboard:        stringPtr("closet"),
	Added:           &time.Time{},
	Expiry:          &time.Time{},
	SafetyDataSheet: stringPtr(""),
	StorageTemp:     "+4",
	IsArchived:      false,
	Hazards:         []string{"Explosive", "Flammable"},
	Owner:           stringPtr(""),
}

var cupboardsChemOne = chemical.Chemical{
	CasNumber:       stringPtr("123456789"),
	Name:            "beans1",
	ChemicalNumber:  stringPtr("blueberries"),
	MatterState:     stringPtr("liquid"),
	Quantity:        stringPtr("5"),
	Location:        stringPtr("Test Lab"),
	Cupboard:        stringPtr("3"),
	Added:           &time.Time{},
	Expiry:          &time.Time{},
	SafetyDataSheet: stringPtr(""),
	StorageTemp:     "+4",
	IsArchived:      false,
	Hazards:         []string{"Explosive", "Flammable"},
	Owner:           stringPtr(""),
}

var cupboardsChemTwo = chemical.Chemical{
	CasNumber:       stringPtr("1234567890"),
	Name:            "beans2",
	ChemicalNumber:  stringPtr("blueberries"),
	MatterState:     stringPtr("liquid"),
	Quantity:        stringPtr("5"),
	Location:        stringPtr("Test Lab"),
	Cupboard:        stringPtr("4"),
	Added:           &time.Time{},
	Expiry:          &time.Time{},
	SafetyDataSheet: stringPtr(""),
	StorageTemp:     "+4",
	IsArchived:      false,
	Hazards:         []string{"Explosive", "Flammable"},
	Owner:           stringPtr(""),
}

func stringPtr(v string) *string {
	return &v
}

var client = &http.Client{}

// Mock response from the JWT validation since the tests don't use the Auth service
var validator = func(audience string, domain string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

func TestMain(m *testing.M) {

	if err := db.Connect(); err != nil {
		log.Fatal("Failed to start DB", err)
	}
	go func() {
		if err := server.Start(":8081", validator); err != nil {
			log.Fatal("Failed to start server", err)
		}
	}()

	// wait for server to start
	time.Sleep(time.Second * 2)

	// InsertTestChemicals()

	status := m.Run()
	os.Exit(status)
}

/*
*
Test fixtures for cupboards
*/
func InsertTestChemicals() {

	_, err := db.InsertChemical(cupboardsChemOne)
	if err != nil {
		log.Fatal("Failed to insert chemical", err)
	}
	_, err = db.InsertChemical(cupboardsChemTwo)
	if err != nil {
		log.Fatal("Failed to insert chemical", err)
	}
}

func TestPostChemical(t *testing.T) {
	jsonChemical, err := json.Marshal(cupboardsChem)
	assert.Nil(t, err, "Failed to marshal into chemical")

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8081/chemical", bytes.NewBuffer(jsonChemical))
	assert.Nil(t, err, "Failed to build post request")

	response, err := client.Do(req)
	assert.Nil(t, err, "Failed to send POST request")
	assert.Equal(t, http.StatusOK, response.StatusCode)

	bodyBytes, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "Failed to read message body")

	var responseChemical chemical.Chemical
	err = json.Unmarshal(bodyBytes, &responseChemical)
	cupboardsChem.Id = responseChemical.Id
	assert.Nil(t, err, "Failed to unmarshal into chemical")
	assert.Equal(t, cupboardsChem, responseChemical)
}

func TestGetChemical(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8081/chemicals", nil)
	assert.Nil(t, err, "Failed to build GET request")

	response, err := client.Do(req)
	assert.Nil(t, err, "Failed to send GET request")

	bodyBytes, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "Failed to read message body")
	var responseChemicals []chemical.Chemical

	err = json.Unmarshal(bodyBytes, &responseChemicals)
	assert.Nil(t, err, "Failed to unmarshal into chemical")
	found := false
	for _, ch := range responseChemicals {
		if reflect.DeepEqual(ch, cupboardsChem) {
			found = true
			break
		}
	}
	assert.True(t, found, "Error: values are not the same")
}

func TestPutChemical(t *testing.T) {
	putChem := cupboardsChem
	putChem.Name = "bread"
	jsonChemical, err := json.Marshal(putChem)
	assert.Nil(t, err, "Failed to marshal into chemical")

	req, err := http.NewRequest(http.MethodPut, "http://localhost:8081/chemical", bytes.NewBuffer(jsonChemical))
	assert.Nil(t, err, "Failed to build PUT request")

	response, err := client.Do(req)
	assert.Nil(t, err, "Failed to send PUT request")

	bodyBytes, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "Failed to read message body")

	var responseChemical chemical.Chemical
	err = json.Unmarshal(bodyBytes, &responseChemical)
	assert.Nil(t, err, "Failed to unmarshal into Chemical")

	assert.Equal(t, putChem, responseChemical)
}

func TestGetCupboards(t *testing.T) {
	expectedResponse := []string{"", "Cupboard 1", "Cupboard 12", "Cupboard 1a", "Cupboard 1b", "Cupboard 2", "Cupboard 3", "Cupboard 4", "Cupboard 7", "closet"}
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8081/cupboards", nil)
	assert.Nil(t, err, "Failed to build GET request")

	response, err := client.Do(req)
	assert.Nil(t, err, "Failed to send GET request")

	bodyBytes, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "Failed to read message body")

	var responseValues []string
	err = json.Unmarshal(bodyBytes, &responseValues)
	assert.Nil(t, err, "Failed to unmarshal into string array")

	assert.Equal(t, expectedResponse, responseValues)

}

func TestGetCupboardsForLab(t *testing.T) {
	expectedResponse := []string{"Cupboard 1", "Cupboard 1a"}
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8081/cupboards", nil)
	q := req.URL.Query()
	q.Add("lab", "Lab 2")
	req.URL.RawQuery = q.Encode()
	assert.Nil(t, err, "Failed to build GET request")

	response, err := client.Do(req)
	assert.Nil(t, err, "Failed to send GET request")

	bodyBytes, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "Failed to read message body")

	var responseValues []string
	err = json.Unmarshal(bodyBytes, &responseValues)
	assert.Nil(t, err, "Failed to unmarshal into string array")

	assert.Equal(t, expectedResponse, responseValues)

}

func TestPutHazards(t *testing.T) {
	putChem := cupboardsChem
	putChem.Hazards = []string{"Corrosive", "Serious health hazard"}
	jsonChemical, err := json.Marshal(putChem)
	assert.Nil(t, err, "Failed to marshal into chemical")

	req, err := http.NewRequest(http.MethodPut, "http://localhost:8081/hazards", bytes.NewBuffer(jsonChemical))
	assert.Nil(t, err, "Failed to build PUT request")

	response, err := client.Do(req)
	assert.Nil(t, err, "Failed to send PUT request")

	bodyBytes, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "Failed to read message body")

	var responseChemical chemical.Chemical
	err = json.Unmarshal(bodyBytes, &responseChemical)
	assert.Nil(t, err, "Failed to unmarshal into Chemical")

	assert.Equal(t, putChem, responseChemical)
}

func TestGetUsers(t *testing.T) {
	expectedResponse := []string{"User One", "User Two"}
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8081/users", nil)
	assert.Nil(t, err, "Failed to build GET request")
	response, err := client.Do(req)
	assert.Nil(t, err, "Failed to send GET request")
	bodyBytes, err := io.ReadAll(response.Body)
	assert.Nil(t, err, "Failed to read message body")
	var responseUserList []string
	err = json.Unmarshal(bodyBytes, &responseUserList)
	assert.Nil(t, err, "Failed to unmarshal into string array")

	assert.Equal(t, expectedResponse, responseUserList)
}
