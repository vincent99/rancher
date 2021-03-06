package fleetcharts

import (
	"context"
	"os"
	"sync"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

var (
	fleetCRDChart = chartDef{
		ReleaseNamespace: "fleet-system",
		ChartName:        "fleet-crd",
	}
	fleetChart = chartDef{
		ReleaseNamespace: "cattle-fleet-system",
		ChartName:        "fleet",
	}
	fleetUninstallChart = chartDef{
		ReleaseNamespace: "fleet-system",
		ChartName:        "fleet",
	}
)

type chartDef struct {
	ReleaseNamespace string
	ChartName        string
}

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager: wContext.SystemChartsManager,
	}

	wContext.Mgmt.Setting().OnChange(ctx, "fleet-install", h.onSetting)
	return nil
}

type handler struct {
	sync.Mutex
	manager *system.Manager
}

func (h *handler) onSetting(key string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil {
		return nil, nil
	}

	if setting.Name != settings.ServerURL.Name &&
		setting.Name != settings.CACerts.Name &&
		setting.Name != settings.SystemDefaultRegistry.Name {
		return setting, nil
	}

	if err := h.manager.Uninstall(fleetUninstallChart.ReleaseNamespace, fleetUninstallChart.ChartName); err != nil {
		return nil, err
	}

	err := h.manager.Ensure(fleetCRDChart.ReleaseNamespace, fleetCRDChart.ChartName, settings.FleetMinVersion.Get(), nil)
	if err != nil {
		return setting, err
	}

	systemGlobalRegistry := map[string]interface{}{
		"cattle": map[string]interface{}{
			"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
		},
	}

	fleetChartValues := map[string]interface{}{
		"apiServerURL": settings.ServerURL.Get(),
		"apiServerCA":  settings.CACerts.Get(),
		"global":       systemGlobalRegistry,
	}

	fleetChartValues["gitops"] = map[string]interface{}{
		"enabled": features.Gitops.Enabled(),
	}

	gitjobChartValues := make(map[string]interface{})

	if envVal, ok := os.LookupEnv("HTTP_PROXY"); ok {
		fleetChartValues["proxy"] = envVal
		gitjobChartValues["proxy"] = envVal
	}
	if envVal, ok := os.LookupEnv("NO_PROXY"); ok {
		fleetChartValues["noProxy"] = envVal
		gitjobChartValues["noProxy"] = envVal
	}

	if len(gitjobChartValues) > 0 {
		fleetChartValues["gitjob"] = gitjobChartValues
	}

	return setting, h.manager.Ensure(fleetChart.ReleaseNamespace, fleetChart.ChartName, settings.FleetMinVersion.Get(), fleetChartValues)
}
