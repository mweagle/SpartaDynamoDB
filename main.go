package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/session"
	sparta "github.com/mweagle/Sparta"
	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
	gocf "github.com/mweagle/go-cloudformation"
)

// Standard AWS λ function
func helloWorldGET(w http.ResponseWriter, r *http.Request) {
	logger, _ := r.Context().Value(sparta.ContextKeyLogger).(*logrus.Logger)
	config, configErr := sparta.Discover()
	if config != nil {
		// Find the DynamoDB table
		for _, eachResourceInfo := range config.Resources {
			if eachResourceInfo.ResourceType == "AWS::DynamoDB::Table" {
				logger.WithFields(logrus.Fields{
					"TableName": eachResourceInfo.ResourceRef,
				}).Info("Dynamic DynamoDB Table")
			}
		}
	} else {
		logger.WithField("Error", configErr).Error("Failed to find Discovery info")
	}
	fmt.Fprint(w, "Hello World GET 🌍")
}

func helloWorldPOST(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello World POST 🌍")
}

func helloWorldDELETE(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello World DELETE 🌍")
}

func helloWorldPUT(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello World PUT 🌍")
}

////////////////////////////////////////////////////////////////////////////////
// Main
func iamRoleDefinitionForMethods(resourceName string, methodNames ...string) sparta.IAMRoleDefinition {
	roleDef := sparta.IAMRoleDefinition{}
	iamRolePermission := sparta.IAMRolePrivilege{}
	iamRolePermission.Resource = gocf.GetAtt(resourceName, "Arn")
	for _, eachMethod := range methodNames {
		switch eachMethod {
		case http.MethodGet:
			iamRolePermission.Actions = append(iamRolePermission.Actions,
				"dynamodb:Query",
				"dynamodb:Get*")
		case http.MethodPost, http.MethodPut:
			iamRolePermission.Actions = append(iamRolePermission.Actions,
				"dynamodb:PutItem",
				"dynamodb:UpdateItem")
		case http.MethodDelete:
			iamRolePermission.Actions = append(iamRolePermission.Actions,
				"dynamodb:DeleteItem")
		}
	}
	roleDef.Privileges = append(roleDef.Privileges, iamRolePermission)
	return roleDef
}

func main() {
	ddbTableResource := sparta.CloudFormationResourceName("DynamoDB", "MyApp")

	// Map of function to their POLAs
	functionMap := map[string]http.HandlerFunc{
		http.MethodGet:    helloWorldGET,
		http.MethodPost:   helloWorldPOST,
		http.MethodDelete: helloWorldDELETE,
		http.MethodPut:    helloWorldPUT,
	}
	var lambdaFunctions []*sparta.LambdaAWSInfo
	for eachMethod, eachFunc := range functionMap {
		iamRoleDef := iamRoleDefinitionForMethods(ddbTableResource, eachMethod)
		lambdaFn := sparta.HandleAWSLambda(sparta.LambdaName(eachFunc),
			http.HandlerFunc(eachFunc),
			iamRoleDef)
		lambdaFn.Options.MemorySize = 128
		lambdaFn.Options.Timeout = 10
		lambdaFn.DependsOn = []string{ddbTableResource}
		lambdaFunctions = append(lambdaFunctions, lambdaFn)
	}

	ddbResourceDecorator := func(context map[string]interface{},
		serviceName string,
		template *gocf.Template,
		S3Bucket string,
		buildID string,
		awsSession *session.Session,
		noop bool,
		logger *logrus.Logger) error {
		ddbResource := gocf.DynamoDBTable{
			ProvisionedThroughput: &gocf.DynamoDBTableProvisionedThroughput{
				ReadCapacityUnits:  gocf.Integer(4),
				WriteCapacityUnits: gocf.Integer(4),
			},
			AttributeDefinitions: &gocf.DynamoDBTableAttributeDefinitionList{
				gocf.DynamoDBTableAttributeDefinition{
					AttributeName: gocf.String("MyKey"),
					AttributeType: gocf.String("S"),
				},
				gocf.DynamoDBTableAttributeDefinition{
					AttributeName: gocf.String("MyValue"),
					AttributeType: gocf.String("S"),
				},
			},
			KeySchema: &gocf.DynamoDBTableKeySchemaList{
				gocf.DynamoDBTableKeySchema{
					AttributeName: gocf.String("MyKey"),
					KeyType:       gocf.String("HASH"),
				},
				gocf.DynamoDBTableKeySchema{
					AttributeName: gocf.String("MyValue"),
					KeyType:       gocf.String("RANGE"),
				},
			},
			StreamSpecification: &gocf.DynamoDBTableStreamSpecification{
				StreamViewType: gocf.String("KEYS_ONLY"),
			},
		}
		ddbResource.Tags = &gocf.TagList{
			gocf.Tag{
				Key:   gocf.String("SomeKey"),
				Value: gocf.String("SomeValue"),
			},
		}
		template.AddResource(ddbTableResource, ddbResource)
		return nil
	}

	// Sanitize the name so that it doesn't have any spaces
	//stackName := spartaCF.UserScopedStackName("SpartaHello")
	workflowHooks := &sparta.WorkflowHooks{
		ServiceDecorator: ddbResourceDecorator,
	}
	stackName := spartaCF.UserScopedStackName("SpartaDDB")
	err := sparta.MainEx(stackName,
		"Simple Sparta application that demonstrates core functionality",
		lambdaFunctions,
		nil,
		nil,
		workflowHooks,
		false)
	if err != nil {
		os.Exit(1)
	}
}
