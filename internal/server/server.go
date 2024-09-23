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

package server

import (
	"bufio"
	"context"
	"encoding/csv"
	"log"
	"net/http"
	"os"

	adapter "github.com/gwatts/gin-adapter"

	"github.com/gin-gonic/gin"
	"github.com/sethvargo/go-envconfig"
	"gitlab.mdcatapult.io/informatics/coshh/coshh-api/internal/chemical"
	"gitlab.mdcatapult.io/informatics/coshh/coshh-api/internal/db"
	"gitlab.mdcatapult.io/informatics/coshh/coshh-api/internal/users"
)

var config Config

type Config struct {
	LabsCSV       string `env:"LABS_CSV,default=/mnt/labs.csv"`
	Auth0Audience string `env:"AUTH0_AUDIENCE,required"`
	Auth0Domain   string `env:"AUTH0_DOMAIN,required"`
	LDAPEnabled   bool   `env:"LDAP_ENABLED, default=false"`
	LDAPUsername  string `env:"LDAP_USERNAME"`
	LDAPPassword  string `env:"LDAP_PASSWORD"`
	UsernameFile  string `env:"USERNAME_FILE,default=/mnt/usernames.txt"`
}

type (
	jwtValidator func(audience string, domain string) func(next http.Handler) http.Handler
)

func Start(port string, validator jwtValidator) *gin.Engine {

	ctx := context.Background()

	if err := envconfig.Process(ctx, &config); err != nil {
		log.Println("Server start env vars unset or incorrect, using default config")
	} else {
		log.Println("Server using config from env vars")
	}
	r := gin.Default()
	r.Use(corsMiddleware())

	log.Printf("Server using Audience=%s, Domain=%s", config.Auth0Audience, config.Auth0Domain)
	r.GET("/chemicals", getChemicals)
	r.PUT("/chemical", adapter.Wrap(validator(config.Auth0Audience, config.Auth0Domain)), updateChemical)

	r.POST("/chemical", adapter.Wrap(validator(config.Auth0Audience, config.Auth0Domain)), insertChemical)

	r.GET("/chemical/maxchemicalnumber", getMaxChemicalNumber)

	r.GET("/cupboards", getCupboards)

	r.PUT("/hazards", adapter.Wrap(validator(config.Auth0Audience, config.Auth0Domain)), updateHazards)

	r.GET("/labs", getLabs)

	//This route is here to allow standalone testing of authentication using curl
	r.GET("/protected", adapter.Wrap(validator(config.Auth0Audience, config.Auth0Domain)), protectedRoute)

	r.GET("/users", getUsers)
	return r
}

func getChemicals(c *gin.Context) {

	chemicals, err := db.SelectAllChemicals()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, chemicals)
}

func getCupboards(c *gin.Context) {
	var chemicals []string
	var err error
	lab := c.Query("lab")
	if lab == "" {
		chemicals, err = db.SelectAllCupboards()
	} else {
		chemicals, err = db.GetCupboardsForLab(lab)
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, chemicals)
}

func updateChemical(c *gin.Context) {

	var chemical chemical.Chemical
	if err := c.BindJSON(&chemical); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.UpdateChemical(chemical); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, chemical)

}

func insertChemical(c *gin.Context) {

	var chemical chemical.Chemical
	if err := c.BindJSON(&chemical); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	id, err := db.InsertChemical(chemical)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	chemical.Id = id

	c.JSON(http.StatusOK, chemical)
}

func updateHazards(c *gin.Context) {
	var chemical chemical.Chemical
	if err := c.BindJSON(&chemical); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := db.DeleteHazards(chemical)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	if len(chemical.Hazards) > 0 {
		err = db.InsertHazards(chemical)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
		}
	}

	c.JSON(http.StatusOK, chemical)
}

func getLabs(c *gin.Context) {

	labsFile, err := os.Open(config.LabsCSV)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	defer labsFile.Close()

	csvReader := csv.NewReader(labsFile)
	labs, err := csvReader.Read()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, labs)
}

// Return list of users from LDAP if enabled or from file containing usernames
func getUsers(c *gin.Context) {
	if config.LDAPEnabled {
		if usersList, err := users.GetUsers(config.LDAPUsername, config.LDAPPassword); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		} else {
			c.JSON(http.StatusOK, usersList)
		}
	} else {
		file, err := os.Open(config.UsernameFile)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer file.Close()

		var userList []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			userList = append(userList, scanner.Text())
		}
		c.JSON(http.StatusOK, userList)
	}
}

func getMaxChemicalNumber(c *gin.Context) {
	chemicalNumber, err := db.GetMaxChemicalNumber()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, chemicalNumber)
}

// Purely for auth testing
func protectedRoute(c *gin.Context) {
	c.JSON(http.StatusOK, "You have successfully authenticated")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST,HEAD,PATCH, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
