package main

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type MetricsCollectorAzureRmResources struct {
	collector.Processor

	prometheus struct {
		resource      *prometheus.GaugeVec
		resourceGroup *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmResources) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.resource = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resource_info",
			Help: "Azure Resource information",
		},
		armclient.AddResourceTagsToPrometheusLabelsDefinition(
			[]string{
				"resourceID",
				"resourceName",
				"subscriptionID",
				"resourceGroup",
				"resourceType",
				"provider",
				"location",
				"provisioningState",
			},
			opts.Azure.ResourceTags,
		),
	)
	prometheus.MustRegister(m.prometheus.resource)

	m.prometheus.resourceGroup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resourcegroup_info",
			Help: "Azure ResourceManager resourcegroup information",
		},
		armclient.AddResourceTagsToPrometheusLabelsDefinition(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"location",
				"provisioningState",
			},
			opts.Azure.ResourceGroupTags,
		),
	)
	prometheus.MustRegister(m.prometheus.resourceGroup)
}

func (m *MetricsCollectorAzureRmResources) Reset() {
	m.prometheus.resource.Reset()
	m.prometheus.resourceGroup.Reset()
}

func (m *MetricsCollectorAzureRmResources) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *log.Entry) {
		m.collectAzureResourceGroup(subscription, logger, callback)
		m.collectAzureResources(subscription, logger, callback)

	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

// Collect Azure ResourceGroup metrics
func (m *MetricsCollectorAzureRmResources) collectAzureResourceGroup(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armresources.NewResourceGroupsClient(*subscription.SubscriptionID, AzureClient.GetCred(), nil)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	pager := client.NewListPager(nil)

	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
		}

		if result.Value == nil {
			continue
		}

		for _, resourceGroup := range result.ResourceGroupListResult.Value {
			resourceId := to.String(resourceGroup.ID)
			azureResource, _ := armclient.ParseResourceId(resourceId)

			infoLabels := prometheus.Labels{
				"resourceID":        to.StringLower(resourceGroup.ID),
				"subscriptionID":    azureResource.Subscription,
				"resourceGroup":     azureResource.ResourceGroup,
				"location":          to.StringLower(resourceGroup.Location),
				"provisioningState": to.StringLower(resourceGroup.Properties.ProvisioningState),
			}
			infoLabels = armclient.AddResourceTagsToPrometheusLabels(infoLabels, resourceGroup.Tags, opts.Azure.ResourceGroupTags)
			infoMetric.AddInfo(infoLabels)
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.resourceGroup)
	}
}

func (m *MetricsCollectorAzureRmResources) collectAzureResources(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armresources.NewClient(*subscription.SubscriptionID, AzureClient.GetCred(), nil)
	if err != nil {
		logger.Panic(err)
	}

	resourceMetric := prometheusCommon.NewMetricsList()

	pager := client.NewListPager(nil)

	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
		}

		if result.Value == nil {
			continue
		}

		for _, resource := range result.ResourceListResult.Value {
			resourceId := to.String(resource.ID)
			azureResource, _ := armclient.ParseResourceId(resourceId)

			infoLabels := prometheus.Labels{
				"subscriptionID":    azureResource.Subscription,
				"resourceID":        stringToStringLower(resourceId),
				"resourceName":      azureResource.ResourceName,
				"resourceGroup":     azureResource.ResourceGroup,
				"provider":          azureResource.ResourceProviderName,
				"resourceType":      azureResource.ResourceType,
				"location":          to.StringLower(resource.Location),
				"provisioningState": to.StringLower(resource.ProvisioningState),
			}
			infoLabels = armclient.AddResourceTagsToPrometheusLabels(infoLabels, resource.Tags, opts.Azure.ResourceTags)
			resourceMetric.AddInfo(infoLabels)
		}
	}

	callback <- func() {
		resourceMetric.GaugeSet(m.prometheus.resource)
	}
}
