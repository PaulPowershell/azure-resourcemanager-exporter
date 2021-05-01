package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/graphrbac/graphrbac"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	"os"
)

type MetricsCollectorGraphApps struct {
	CollectorProcessorCustom

	client *graphrbac.ApplicationsClient

	prometheus struct {
		apps            *prometheus.GaugeVec
		appsCredentials *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorGraphApps) Setup(collector *CollectorCustom) {
	m.CollectorReference = collector

	// init azure client
	auth, _ := auth.NewAuthorizerFromEnvironmentWithResource(azureEnvironment.GraphEndpoint)
	client := graphrbac.NewApplicationsClient(os.Getenv("AZURE_TENANT_ID"))
	client.Authorizer = auth
	client.ResponseInspector = azureResponseInspector(nil)

	m.client = &client

	m.prometheus.apps = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_app_info",
			Help: "Azure GraphQL applications",
		},
		[]string{
			"appAppID",
			"appObjectID",
			"appDisplayName",
			"appObjectType",
		},
	)

	m.prometheus.appsCredentials = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_app_credential",
			Help: "Azure GraphQL application credentials",
		},
		[]string{
			"appAppID",
			"credentialID",
			"credentialType",
			"type",
		},
	)

	prometheus.MustRegister(m.prometheus.apps)
	prometheus.MustRegister(m.prometheus.appsCredentials)
}

func (m *MetricsCollectorGraphApps) Collect(ctx context.Context, logger *log.Entry) {
	appsMetrics := prometheusCommon.NewMetricsList()
	appsCredentialMetrics := prometheusCommon.NewMetricsList()

	list, err := m.client.List(context.Background(), opts.GraphApplicationFilter)
	if err != nil {
		logger.Panic(err)
	}

	for _, row := range list.Values() {
		appsMetrics.AddInfo(prometheus.Labels{
			"appAppID":       stringPtrToString(row.AppID),
			"appObjectID":    stringPtrToString(row.ObjectID),
			"appDisplayName": stringPtrToString(row.DisplayName),
			"appObjectType":  string(row.ObjectType),
		})

		// password credentials
		if row.PasswordCredentials != nil {
			for _, credential := range *row.PasswordCredentials {
				if credential.StartDate != nil {
					appsCredentialMetrics.AddTime(prometheus.Labels{
						"appAppID":       stringPtrToString(row.AppID),
						"credentialID":   stringPtrToString(credential.KeyID),
						"credentialType": "password",
						"type":           "startDate",
					}, (*credential.StartDate).ToTime())
				}

				if credential.EndDate != nil {
					appsCredentialMetrics.AddTime(prometheus.Labels{
						"appAppID":       stringPtrToString(row.AppID),
						"credentialID":   stringPtrToString(credential.KeyID),
						"credentialType": "password",
						"type":           "endDate",
					}, (*credential.EndDate).ToTime())
				}
			}
		}

		// key credentials
		if row.KeyCredentials != nil {
			for _, credential := range *row.KeyCredentials {
				if credential.StartDate != nil {
					appsCredentialMetrics.AddTime(prometheus.Labels{
						"appAppID":       stringPtrToString(row.AppID),
						"credentialID":   stringPtrToString(credential.KeyID),
						"credentialType": "key",
						"type":           "startDate",
					}, (*credential.StartDate).ToTime())
				}

				if credential.EndDate != nil {
					appsCredentialMetrics.AddTime(prometheus.Labels{
						"appAppID":       stringPtrToString(row.AppID),
						"credentialID":   stringPtrToString(credential.KeyID),
						"credentialType": "key",
						"type":           "endDate",
					}, (*credential.EndDate).ToTime())
				}
			}
		}
	}

	m.prometheus.apps.Reset()
	m.prometheus.appsCredentials.Reset()
	appsMetrics.GaugeSet(m.prometheus.apps)
	appsCredentialMetrics.GaugeSet(m.prometheus.appsCredentials)
}
