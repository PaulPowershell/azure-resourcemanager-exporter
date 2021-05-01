package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/mysql/mgmt/mysql"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/postgresql/mgmt/postgresql"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
)

type MetricsCollectorAzureRmDatabase struct {
	CollectorProcessorGeneral

	prometheus struct {
		database       *prometheus.GaugeVec
		databaseStatus *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmDatabase) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.database = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_database_info",
			Help: "Azure Database info",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"location",
				"type",
				"serverName",
				"resourceGroup",
				"version",
				"skuName",
				"skuTier",
				"fqdn",
				"sslEnforcement",
				"geoRedundantBackup",
			},
			azureResourceTags.prometheusLabels...,
		),
	)

	m.prometheus.databaseStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_database_status",
			Help: "Azure Database status informations",
		},
		[]string{
			"resourceID",
			"type",
		},
	)

	prometheus.MustRegister(m.prometheus.database)
	prometheus.MustRegister(m.prometheus.databaseStatus)
}

func (m *MetricsCollectorAzureRmDatabase) Reset() {
	m.prometheus.database.Reset()
	m.prometheus.databaseStatus.Reset()
}

func (m *MetricsCollectorAzureRmDatabase) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureDatabasePostgresql(ctx, logger, callback, subscription)
	m.collectAzureDatabaseMysql(ctx, logger, callback, subscription)
}

func (m *MetricsCollectorAzureRmDatabase) collectAzureDatabasePostgresql(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := postgresql.NewServersClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.List(ctx)

	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()
	statusMetric := prometheusCommon.NewMetricsList()

	for _, val := range *list.Value {
		skuName := ""
		skuTier := ""

		if val.Sku != nil {
			skuName = string(*val.Sku.Name)
			skuTier = string(val.Sku.Tier)
		}

		infoLabels := prometheus.Labels{
			"resourceID":         *val.ID,
			"subscriptionID":     *subscription.SubscriptionID,
			"location":           *val.Location,
			"type":               "postgresql",
			"serverName":         *val.Name,
			"resourceGroup":      extractResourceGroupFromAzureId(*val.ID),
			"skuName":            skuName,
			"skuTier":            skuTier,
			"version":            string(val.Version),
			"fqdn":               *val.FullyQualifiedDomainName,
			"sslEnforcement":     string(val.SslEnforcement),
			"geoRedundantBackup": string(val.StorageProfile.GeoRedundantBackup),
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.Add(infoLabels, 1)

		statusMetric.Add(prometheus.Labels{
			"resourceID": *val.ID,
			"type":       "backupRetentionDays",
		}, float64(*val.StorageProfile.BackupRetentionDays))

		if val.EarliestRestoreDate != nil {
			statusMetric.AddTime(prometheus.Labels{
				"resourceID": *val.ID,
				"type":       "earliestRestoreDate",
			}, val.EarliestRestoreDate.ToTime())
		}

		if val.ReplicaCapacity != nil {
			statusMetric.Add(prometheus.Labels{
				"resourceID": *val.ID,
				"type":       "replicaCapacity",
			}, float64(*val.ReplicaCapacity))
		}

		statusMetric.Add(prometheus.Labels{
			"resourceID": *val.ID,
			"type":       "storage",
		}, float64(*val.StorageProfile.StorageMB)*1048576)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.database)
		statusMetric.GaugeSet(m.prometheus.databaseStatus)
	}
}

func (m *MetricsCollectorAzureRmDatabase) collectAzureDatabaseMysql(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := mysql.NewServersClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	list, err := client.List(ctx)

	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()
	statusMetric := prometheusCommon.NewMetricsList()

	for _, val := range *list.Value {
		skuName := ""
		skuTier := ""

		if val.Sku != nil {
			skuName = stringPtrToString(val.Sku.Name)
			skuTier = string(val.Sku.Tier)
		}

		infoLabels := prometheus.Labels{
			"resourceID":         *val.ID,
			"subscriptionID":     *subscription.SubscriptionID,
			"location":           stringPtrToString(val.Location),
			"serverName":         stringPtrToString(val.Name),
			"type":               "mysql",
			"resourceGroup":      extractResourceGroupFromAzureId(*val.ID),
			"skuName":            skuName,
			"skuTier":            skuTier,
			"version":            string(val.Version),
			"fqdn":               *val.FullyQualifiedDomainName,
			"sslEnforcement":     string(val.SslEnforcement),
			"geoRedundantBackup": string(val.StorageProfile.GeoRedundantBackup),
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.AddInfo(infoLabels)

		statusMetric.Add(prometheus.Labels{
			"resourceID": *val.ID,
			"type":       "backupRetentionDays",
		}, float64(*val.StorageProfile.BackupRetentionDays))

		if val.EarliestRestoreDate != nil {
			statusMetric.AddTime(prometheus.Labels{
				"resourceID": *val.ID,
				"type":       "earliestRestoreDate",
			}, val.EarliestRestoreDate.ToTime())
		}

		if val.ReplicaCapacity != nil {
			statusMetric.Add(prometheus.Labels{
				"resourceID": *val.ID,
				"type":       "replicaCapacity",
			}, float64(*val.ReplicaCapacity))
		}

		statusMetric.Add(prometheus.Labels{
			"resourceID": *val.ID,
			"type":       "storage",
		}, float64(*val.StorageProfile.StorageMB)*1048576)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.database)
		statusMetric.GaugeSet(m.prometheus.databaseStatus)
	}
}
