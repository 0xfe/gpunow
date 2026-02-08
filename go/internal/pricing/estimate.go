package pricing

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	cloudbilling "google.golang.org/api/cloudbilling/v1"
)

type Request struct {
	Currency          string
	Zone              string
	MachineType       string
	VCPU              int64
	MemoryMB          int64
	ProvisioningModel string
	GPUType           string
	GPUCount          int
	DiskType          string
	DiskSizeGB        int
	NumInstances      int
	MaxRunHours       int
	Refresh           bool
}

type Result struct {
	Currency       string
	CachePath      string
	FetchedSKUs    bool
	TotalPerHour   float64
	TotalForMaxRun float64
	MaxRunHours    int
	NumInstances   int
	Components     []Component
}

type Component struct {
	Name                string
	Key                 string
	SKUID               string
	SKUDescription      string
	QuantityPerInstance float64
	QuantityUnit        string
	UsageUnit           string
	UnitPrice           float64
	CostPerHour         float64
	CostForMaxRun       float64
}

type Estimator struct {
	Cache   *CacheStore
	Catalog Catalog
	nowFn   func() time.Time
}

func NewEstimator(cache *CacheStore, catalog Catalog) *Estimator {
	return &Estimator{
		Cache:   cache,
		Catalog: catalog,
		nowFn:   func() time.Time { return time.Now().UTC() },
	}
}

func (e *Estimator) Estimate(ctx context.Context, req Request) (*Result, error) {
	if e == nil {
		return nil, fmt.Errorf("estimator is nil")
	}
	if e.Cache == nil {
		return nil, fmt.Errorf("pricing cache store is required")
	}
	if e.Catalog == nil {
		return nil, fmt.Errorf("pricing catalog is required")
	}
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "USD"
	}

	cacheData, err := e.Cache.Load()
	if err != nil {
		return nil, err
	}
	if cacheData.Currency == "" {
		cacheData.Currency = currency
	}

	selectors, err := buildSelectors(req)
	if err != nil {
		return nil, err
	}

	resolved := map[string]*CacheEntry{}
	toResolve := make([]skuSelector, 0, len(selectors))
	for _, sel := range selectors {
		if !req.Refresh {
			if entry := cacheData.Entries[sel.Key]; validCacheEntry(entry, currency) {
				resolved[sel.Key] = entry
				continue
			}
		}
		toResolve = append(toResolve, sel)
	}

	fetched := false
	if len(toResolve) > 0 {
		skus, err := e.Catalog.ListComputeSKUs(ctx, currency)
		if err != nil {
			return nil, err
		}
		fetched = true

		now := e.nowFn().Format(time.RFC3339)
		for _, sel := range toResolve {
			entry, err := resolveSelector(skus, sel, currency)
			if err != nil {
				return nil, err
			}
			entry.Key = sel.Key
			entry.Region = sel.Region
			entry.Currency = currency
			entry.FetchedAt = now

			cacheData.Entries[sel.Key] = entry
			resolved[sel.Key] = entry
		}
		cacheData.Currency = currency
		cacheData.UpdatedAt = now
		if err := e.Cache.Save(cacheData); err != nil {
			return nil, err
		}
	}

	result := &Result{
		Currency:     currency,
		CachePath:    e.Cache.Path,
		FetchedSKUs:  fetched,
		MaxRunHours:  req.MaxRunHours,
		NumInstances: req.NumInstances,
		Components:   make([]Component, 0, len(selectors)),
	}

	for _, sel := range selectors {
		entry := resolved[sel.Key]
		if entry == nil {
			return nil, fmt.Errorf("pricing data missing for %s", sel.Name)
		}
		hourlyMultiplier, err := hourlyMultiplierForUsageUnit(entry.Unit)
		if err != nil {
			return nil, fmt.Errorf("unsupported pricing unit for %s (%s): %w", sel.Name, entry.Unit, err)
		}

		quantityTotal := sel.QuantityPerInstance * float64(req.NumInstances)
		perHour := entry.UnitPrice * quantityTotal * hourlyMultiplier
		perRun := perHour * float64(req.MaxRunHours)

		result.TotalPerHour += perHour
		result.TotalForMaxRun += perRun
		result.Components = append(result.Components, Component{
			Name:                sel.Name,
			Key:                 sel.Key,
			SKUID:               entry.SKUID,
			SKUDescription:      entry.Description,
			QuantityPerInstance: sel.QuantityPerInstance,
			QuantityUnit:        sel.QuantityUnit,
			UsageUnit:           entry.Unit,
			UnitPrice:           entry.UnitPrice,
			CostPerHour:         perHour,
			CostForMaxRun:       perRun,
		})
	}

	return result, nil
}

func validateRequest(req Request) error {
	if strings.TrimSpace(req.Zone) == "" {
		return fmt.Errorf("zone is required")
	}
	if strings.TrimSpace(req.MachineType) == "" {
		return fmt.Errorf("machine type is required")
	}
	if req.VCPU <= 0 {
		return fmt.Errorf("vCPU count must be positive")
	}
	if req.MemoryMB <= 0 {
		return fmt.Errorf("memory MB must be positive")
	}
	gpuType := strings.TrimSpace(req.GPUType)
	if gpuType == "" && req.GPUCount > 0 {
		return fmt.Errorf("gpu type is required when gpu count is set")
	}
	if gpuType != "" && req.GPUCount <= 0 {
		return fmt.Errorf("gpu count must be positive when gpu type is set")
	}
	if req.GPUCount < 0 {
		return fmt.Errorf("gpu count cannot be negative")
	}
	if strings.TrimSpace(req.DiskType) == "" {
		return fmt.Errorf("disk type is required")
	}
	if req.DiskSizeGB <= 0 {
		return fmt.Errorf("disk size must be positive")
	}
	if req.NumInstances <= 0 {
		return fmt.Errorf("num instances must be positive")
	}
	if req.MaxRunHours <= 0 {
		return fmt.Errorf("max run hours must be positive")
	}
	return nil
}

func validCacheEntry(entry *CacheEntry, currency string) bool {
	if entry == nil {
		return false
	}
	if strings.TrimSpace(entry.Unit) == "" {
		return false
	}
	if entry.UnitPrice <= 0 {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(entry.Currency), strings.TrimSpace(currency)) {
		return false
	}
	return true
}

type usageExpectation int

const (
	usageAny usageExpectation = iota
	usageSpot
	usageOnDemand
)

func usageFromProvisioningModel(value string) usageExpectation {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SPOT":
		return usageSpot
	case "STANDARD":
		return usageOnDemand
	default:
		return usageAny
	}
}

type skuSelector struct {
	Key                 string
	Name                string
	Region              string
	ResourceFamily      string
	ResourceGroup       string
	Usage               usageExpectation
	RequiredTokens      []string
	ForbiddenTokens     []string
	QuantityPerInstance float64
	QuantityUnit        string
}

func buildSelectors(req Request) ([]skuSelector, error) {
	region, err := regionFromZone(req.Zone)
	if err != nil {
		return nil, err
	}
	family, custom := machineFamily(req.MachineType)
	if family == "" {
		return nil, fmt.Errorf("unable to derive machine family from %s", req.MachineType)
	}
	usage := usageFromProvisioningModel(req.ProvisioningModel)
	familyToken := strings.ToLower(family)
	memoryGiB := float64(req.MemoryMB) / 1024.0

	coreTokens := []string{familyToken, "instance", "core"}
	ramTokens := []string{familyToken, "instance", "ram"}
	coreForbiddenTokens := []string{"sole tenancy", "commitment", "committed use", "reservation"}
	ramForbiddenTokens := []string{"sole tenancy", "commitment", "committed use", "reservation"}
	if custom {
		coreTokens = append(coreTokens, "custom")
		ramTokens = append(ramTokens, "custom")
	} else {
		coreForbiddenTokens = append(coreForbiddenTokens, "custom")
		ramForbiddenTokens = append(ramForbiddenTokens, "custom")
	}

	diskGroup, diskTokens, diskForbiddenTokens := diskSelectorCriteria(req.DiskType)
	if len(diskForbiddenTokens) == 0 {
		diskForbiddenTokens = []string{"snapshot", "egress", "operation"}
	}

	machineKey := normalizeKeyPart(req.MachineType)
	diskKey := normalizeKeyPart(req.DiskType)

	selectors := []skuSelector{
		{
			Key:                 fmt.Sprintf("compute.core.%s.%s.%s", machineKey, usageKey(usage), region),
			Name:                "vCPU",
			Region:              region,
			ResourceFamily:      "Compute",
			ResourceGroup:       "CPU",
			Usage:               usage,
			RequiredTokens:      coreTokens,
			ForbiddenTokens:     coreForbiddenTokens,
			QuantityPerInstance: float64(req.VCPU),
			QuantityUnit:        "vCPU",
		},
		{
			Key:                 fmt.Sprintf("compute.ram.%s.%s.%s", machineKey, usageKey(usage), region),
			Name:                "RAM",
			Region:              region,
			ResourceFamily:      "Compute",
			ResourceGroup:       "RAM",
			Usage:               usage,
			RequiredTokens:      ramTokens,
			ForbiddenTokens:     ramForbiddenTokens,
			QuantityPerInstance: memoryGiB,
			QuantityUnit:        "GiB",
		},
		{
			Key:                 fmt.Sprintf("compute.disk.%s.%s", diskKey, region),
			Name:                "Disk",
			Region:              region,
			ResourceFamily:      "Storage",
			ResourceGroup:       diskGroup,
			Usage:               usageAny,
			RequiredTokens:      diskTokens,
			ForbiddenTokens:     diskForbiddenTokens,
			QuantityPerInstance: float64(req.DiskSizeGB),
			QuantityUnit:        "GiB",
		},
	}
	if gpuType := strings.TrimSpace(req.GPUType); gpuType != "" && req.GPUCount > 0 {
		gpuTokens := gpuDescriptionTokens(gpuType)
		gpuTokens = append(gpuTokens, "gpu")
		gpuKey := normalizeKeyPart(gpuType)
		selectors = append(selectors, skuSelector{
			Key:                 fmt.Sprintf("compute.gpu.%s.%s.%s", gpuKey, usageKey(usage), region),
			Name:                "GPU",
			Region:              region,
			ResourceFamily:      "Compute",
			ResourceGroup:       "GPU",
			Usage:               usage,
			RequiredTokens:      gpuTokens,
			ForbiddenTokens:     []string{"sole tenancy", "commitment", "committed use"},
			QuantityPerInstance: float64(req.GPUCount),
			QuantityUnit:        "GPU",
		})
	}
	return selectors, nil
}

func usageKey(value usageExpectation) string {
	switch value {
	case usageSpot:
		return "spot"
	case usageOnDemand:
		return "ondemand"
	default:
		return "any"
	}
}

func resolveSelector(skus []*cloudbilling.Sku, sel skuSelector, currency string) (*CacheEntry, error) {
	type candidate struct {
		entry *CacheEntry
		score int
	}

	candidates := []candidate{}
	for _, sku := range skus {
		if sku == nil || sku.Category == nil {
			continue
		}
		if sel.ResourceFamily != "" && !strings.EqualFold(sku.Category.ResourceFamily, sel.ResourceFamily) {
			continue
		}
		if sel.ResourceGroup != "" && !strings.EqualFold(sku.Category.ResourceGroup, sel.ResourceGroup) {
			continue
		}
		if !skuMatchesRegion(sku, sel.Region) {
			continue
		}
		usageScore := usageMatchScore(sku.Category.UsageType, sel.Usage)
		if usageScore == 0 {
			continue
		}

		description := strings.ToLower(strings.TrimSpace(sku.Description))
		if !containsAll(description, sel.RequiredTokens) {
			continue
		}
		if containsAny(description, sel.ForbiddenTokens) {
			continue
		}

		entry, err := cacheEntryFromSKU(sku, currency)
		if err != nil {
			continue
		}
		entry.ResourceFamily = sku.Category.ResourceFamily
		entry.ResourceGroup = sku.Category.ResourceGroup
		entry.UsageType = sku.Category.UsageType

		score := 0
		if strings.Contains(description, "running in "+strings.ToLower(sel.Region)) {
			score += 2
		}
		if sel.ResourceGroup != "" && strings.EqualFold(sku.Category.ResourceGroup, sel.ResourceGroup) {
			score++
		}
		score += usageScore
		candidates = append(candidates, candidate{entry: entry, score: score})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no pricing SKU match for %s (%s)", sel.Name, sel.Key)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].entry.SKUID < candidates[j].entry.SKUID
		}
		return candidates[i].score > candidates[j].score
	})
	best := candidates[0]
	if len(candidates) > 1 && candidates[1].score == best.score && candidates[1].entry.SKUID != best.entry.SKUID {
		skus := []string{best.entry.SKUID, candidates[1].entry.SKUID}
		return nil, fmt.Errorf("ambiguous pricing SKU match for %s (%s): %s", sel.Name, sel.Key, strings.Join(skus, ", "))
	}
	return best.entry, nil
}

func cacheEntryFromSKU(sku *cloudbilling.Sku, currency string) (*CacheEntry, error) {
	price, unit, effectiveTime, err := latestUnitPrice(sku)
	if err != nil {
		return nil, err
	}
	return &CacheEntry{
		SKUName:       sku.Name,
		SKUID:         sku.SkuId,
		Description:   sku.Description,
		Unit:          unit,
		UnitPrice:     price,
		EffectiveTime: effectiveTime,
		Currency:      currency,
	}, nil
}

func latestUnitPrice(sku *cloudbilling.Sku) (float64, string, string, error) {
	if sku == nil || len(sku.PricingInfo) == 0 {
		return 0, "", "", fmt.Errorf("pricing info unavailable")
	}

	latest := sku.PricingInfo[0]
	latestTime := parseTimeOrZero(latest.EffectiveTime)
	for _, info := range sku.PricingInfo[1:] {
		infoTime := parseTimeOrZero(info.EffectiveTime)
		if infoTime.After(latestTime) {
			latest = info
			latestTime = infoTime
		}
	}
	if latest == nil || latest.PricingExpression == nil {
		return 0, "", "", fmt.Errorf("pricing expression unavailable")
	}
	expr := latest.PricingExpression
	if len(expr.TieredRates) == 0 {
		return 0, "", "", fmt.Errorf("tiered rates unavailable")
	}

	var pricedTier *cloudbilling.TierRate
	for _, rate := range expr.TieredRates {
		if rate == nil || rate.UnitPrice == nil {
			continue
		}
		if moneyToFloat(rate.UnitPrice) <= 0 {
			continue
		}
		if pricedTier == nil || rate.StartUsageAmount < pricedTier.StartUsageAmount {
			pricedTier = rate
		}
	}
	if pricedTier == nil {
		return 0, "", "", fmt.Errorf("unit price unavailable")
	}

	price := moneyToFloat(pricedTier.UnitPrice)
	if price <= 0 {
		return 0, "", "", fmt.Errorf("non-positive unit price")
	}
	unit := strings.TrimSpace(expr.UsageUnit)
	if unit == "" {
		unit = strings.TrimSpace(expr.BaseUnit)
	}
	if unit == "" {
		return 0, "", "", fmt.Errorf("usage unit unavailable")
	}
	return price, unit, latest.EffectiveTime, nil
}

func moneyToFloat(value *cloudbilling.Money) float64 {
	if value == nil {
		return 0
	}
	return float64(value.Units) + float64(value.Nanos)/1e9
}

func hourlyMultiplierForUsageUnit(unit string) (float64, error) {
	u := strings.ToLower(strings.TrimSpace(unit))
	u = strings.ReplaceAll(u, " ", "")
	switch {
	case u == "h", strings.Contains(u, "hour"), strings.HasSuffix(u, ".h"), strings.Contains(u, "/h"):
		return 1.0, nil
	case u == "s", strings.Contains(u, "second"), strings.HasSuffix(u, ".s"), strings.Contains(u, "/s"):
		return 3600.0, nil
	case strings.Contains(u, "month"), strings.HasSuffix(u, ".mo"), strings.Contains(u, "/mo"), u == "mo":
		return 1.0 / 730.0, nil
	default:
		return 0, fmt.Errorf("unit %q is not recognized", unit)
	}
}

func skuMatchesRegion(sku *cloudbilling.Sku, region string) bool {
	if sku == nil {
		return false
	}
	for _, r := range sku.ServiceRegions {
		if strings.EqualFold(strings.TrimSpace(r), strings.TrimSpace(region)) {
			return true
		}
	}
	if sku.GeoTaxonomy != nil && strings.EqualFold(sku.GeoTaxonomy.Type, "GLOBAL") {
		return true
	}
	return false
}

func skuMatchesUsage(actual string, expected usageExpectation) bool {
	return usageMatchScore(actual, expected) > 0
}

func usageMatchScore(actual string, expected usageExpectation) int {
	actualValue := strings.ToLower(strings.TrimSpace(actual))
	switch expected {
	case usageSpot:
		switch actualValue {
		case "spot":
			return 3
		case "preemptible":
			return 2
		default:
			return 0
		}
	case usageOnDemand:
		switch actualValue {
		case "ondemand", "on demand":
			return 3
		default:
			return 0
		}
	default:
		if actualValue == "" {
			return 1
		}
		return 2
	}
}

func containsAll(text string, tokens []string) bool {
	for _, token := range tokens {
		t := strings.ToLower(strings.TrimSpace(token))
		if t == "" {
			continue
		}
		if !strings.Contains(text, t) {
			return false
		}
	}
	return true
}

func containsAny(text string, tokens []string) bool {
	for _, token := range tokens {
		t := strings.ToLower(strings.TrimSpace(token))
		if t == "" {
			continue
		}
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}

func machineFamily(machineType string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(machineType))
	if value == "" {
		return "", false
	}
	if strings.HasPrefix(value, "custom-") {
		return "n1", true
	}
	if idx := strings.Index(value, "-custom-"); idx > 0 {
		return value[:idx], true
	}
	parts := strings.Split(value, "-")
	if len(parts) == 0 {
		return "", false
	}
	return parts[0], false
}

func regionFromZone(zone string) (string, error) {
	parts := strings.Split(strings.TrimSpace(zone), "-")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid zone: %s", zone)
	}
	return strings.Join(parts[:2], "-"), nil
}

func gpuDescriptionTokens(gpuType string) []string {
	normalized := strings.ToLower(strings.TrimSpace(gpuType))
	if normalized == "" {
		return nil
	}
	normalized = strings.TrimPrefix(normalized, "nvidia-")
	parts := strings.Fields(strings.ReplaceAll(normalized, "-", " "))
	tokens := make([]string, 0, len(parts)+1)
	tokens = append(tokens, "nvidia")
	for _, part := range parts {
		if part != "" {
			tokens = append(tokens, part)
		}
	}
	return tokens
}

func diskDescriptionTokens(diskType string) []string {
	switch strings.ToLower(strings.TrimSpace(diskType)) {
	case "pd-standard":
		return []string{"standard", "persistent", "disk"}
	case "pd-balanced":
		return []string{"balanced", "persistent", "disk"}
	case "pd-ssd":
		return []string{"ssd", "persistent", "disk"}
	case "hyperdisk-balanced":
		return []string{"hyperdisk", "balanced"}
	case "hyperdisk-throughput":
		return []string{"hyperdisk", "throughput"}
	case "hyperdisk-extreme":
		return []string{"hyperdisk", "extreme"}
	case "local-ssd":
		return []string{"local", "ssd"}
	default:
		value := strings.ToLower(strings.TrimSpace(diskType))
		value = strings.ReplaceAll(value, "_", "-")
		parts := strings.Fields(strings.ReplaceAll(value, "-", " "))
		if len(parts) == 0 {
			return []string{"disk"}
		}
		tokens := make([]string, 0, len(parts)+1)
		tokens = append(tokens, parts...)
		if !containsAny(strings.Join(tokens, " "), []string{"disk", "ssd", "pd"}) {
			tokens = append(tokens, "disk")
		}
		return tokens
	}
}

func diskSelectorCriteria(diskType string) (resourceGroup string, required []string, forbidden []string) {
	switch strings.ToLower(strings.TrimSpace(diskType)) {
	case "pd-standard":
		return "PDStandard",
			[]string{"storage", "pd", "capacity"},
			[]string{"snapshot", "instant snapshot", "egress", "operation", "regional"}
	case "pd-balanced":
		return "SSD",
			[]string{"balanced", "pd", "capacity"},
			[]string{"snapshot", "instant snapshot", "egress", "operation", "regional", "hyperdisk", "asynchronous replication", "storage pools", "high availability", "confidential mode"}
	case "pd-ssd":
		return "SSD",
			[]string{"ssd", "pd", "capacity"},
			[]string{"snapshot", "instant snapshot", "egress", "operation", "regional", "hyperdisk", "asynchronous replication", "storage pools", "high availability", "confidential mode", "balanced"}
	case "hyperdisk-balanced":
		return "SSD",
			[]string{"hyperdisk", "balanced", "capacity"},
			[]string{"snapshot", "instant snapshot", "egress", "operation", "asynchronous replication", "storage pools", "high availability", "confidential mode"}
	case "hyperdisk-throughput":
		return "SSD",
			[]string{"hyperdisk", "throughput", "capacity"},
			[]string{"snapshot", "instant snapshot", "egress", "operation", "asynchronous replication", "storage pools", "confidential mode"}
	case "hyperdisk-extreme":
		return "SSD",
			[]string{"hyperdisk", "extreme", "capacity"},
			[]string{"snapshot", "instant snapshot", "egress", "operation", "asynchronous replication", "storage pools", "confidential mode"}
	default:
		return "", diskDescriptionTokens(diskType), []string{"snapshot", "egress", "operation"}
	}
}

func normalizeKeyPart(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "/", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, ".", "-")
	normalized = strings.ReplaceAll(normalized, " ", "-")
	return normalized
}

func parseTimeOrZero(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
