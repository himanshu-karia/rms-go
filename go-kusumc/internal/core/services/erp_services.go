package services

import (
	"ingestion-go/internal/adapters/secondary"
)

// --- MAINTENANCE ---
type MaintenanceService struct {
	repo *secondary.PostgresRepo
}

func NewMaintenanceService(repo *secondary.PostgresRepo) *MaintenanceService {
	return &MaintenanceService{repo: repo}
}

func (s *MaintenanceService) GetWorkOrders() ([]map[string]interface{}, error) {
	return s.repo.GetWorkOrders()
}

// --- INVENTORY ---
type InventoryService struct {
	repo *secondary.PostgresRepo
}

func NewInventoryService(repo *secondary.PostgresRepo) *InventoryService {
	return &InventoryService{repo: repo}
}

func (s *InventoryService) GetProducts() ([]map[string]interface{}, error) {
	return s.repo.GetProducts()
}

func (s *InventoryService) GetStockLevels(locationId string) ([]map[string]interface{}, error) {
	return s.repo.GetStockLevels(locationId)
}

// --- LOGISTICS ---
type LogisticsService struct {
	repo *secondary.PostgresRepo
}

func NewLogisticsService(repo *secondary.PostgresRepo) *LogisticsService {
	return &LogisticsService{repo: repo}
}

func (s *LogisticsService) GetTrips() ([]map[string]interface{}, error) {
	return s.repo.GetTrips()
}

func (s *LogisticsService) GetGeofences() ([]map[string]interface{}, error) {
	return s.repo.GetGeofences()
}

func (s *LogisticsService) GetAssetTimeline(assetId string) ([]map[string]interface{}, error) {
	return s.repo.GetAssetTimeline(assetId)
}

// --- TRAFFIC ---
type TrafficService struct {
	repo *secondary.PostgresRepo
}

func NewTrafficService(repo *secondary.PostgresRepo) *TrafficService {
	return &TrafficService{repo: repo}
}

func (s *TrafficService) GetCameras() ([]map[string]interface{}, error) {
	return s.repo.GetTrafficCameras()
}

func (s *TrafficService) GetMetrics(deviceId string, limit int) ([]map[string]interface{}, error) {
	return s.repo.GetTrafficMetrics(deviceId, limit)
}

func (s *TrafficService) CreateMetric(deviceId string, breakdown map[string]interface{}, avgSpeed float64, congestion string, vehicleCount int) error {
	return s.repo.CreateTrafficMetric(deviceId, breakdown, avgSpeed, congestion, vehicleCount)
}

// --- WRITE OPERATIONS (Phase 7) ---

func (s *MaintenanceService) CreateWorkOrder(wo map[string]interface{}) error {
	return s.repo.CreateWorkOrder(wo)
}

func (s *MaintenanceService) ResolveWorkOrder(id, notes string) error {
	return s.repo.ResolveWorkOrder(id, notes)
}

func (s *InventoryService) CreateProduct(prod map[string]interface{}) error {
	return s.repo.CreateProduct(prod)
}

func (s *LogisticsService) CreateTrip(trip map[string]interface{}) error {
	return s.repo.CreateTrip(trip)
}
