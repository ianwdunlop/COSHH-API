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
	"context"

	"github.com/auth0-developer-hub/api_standard-library_golang_hello-world/pkg/middleware"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	_ "github.com/lib/pq"
	"gitlab.mdcatapult.io/informatics/coshh/coshh-api/internal/db"
	"gitlab.mdcatapult.io/informatics/coshh/coshh-api/internal/server"

	"log"
)

var ginLambda *ginadapter.GinLambda

func main() {

	if err := db.Connect(); err != nil {
		log.Fatal("Failed to start DB", err)
	}
	ginLambda = ginadapter.New(server.Start(":8080", middleware.ValidateJWT))
	lambda.Start(Handler)

}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// If no name is provided in the HTTP request body, throw an error
	return ginLambda.ProxyWithContext(ctx, req)
}
