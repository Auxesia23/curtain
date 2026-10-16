package services

import (
	"curtain/internal/dto"
	"curtain/internal/models"

	"gorm.io/gorm"
)

type VariableService struct {
	db *gorm.DB
}

func NewVariableService(db *gorm.DB) *VariableService {
	return &VariableService{db: db}
}

func (s *VariableService) Create(envID uint, req dto.UpsertVariableRequest) (models.EnvironmentVariable, error) {
	v := models.EnvironmentVariable{
		EnvironmentID:  envID,
		EncryptedKey:   req.EncryptedKey,
		EncryptedValue: req.EncryptedValue,
	}
	if err := s.db.Create(&v).Error; err != nil {
		return models.EnvironmentVariable{}, err
	}
	return v, nil
}

func (s *VariableService) ListByEnvironment(envID uint) ([]models.EnvironmentVariable, error) {
	var vars []models.EnvironmentVariable
	if err := s.db.Where("environment_id = ?", envID).Order("id ASC").Find(&vars).Error; err != nil {
		return nil, err
	}
	if vars == nil {
		vars = []models.EnvironmentVariable{}
	}
	return vars, nil
}

func (s *VariableService) Update(variableID uint, req dto.UpsertVariableRequest) (models.EnvironmentVariable, error) {
	var v models.EnvironmentVariable
	if err := s.db.First(&v, variableID).Error; err != nil {
		return models.EnvironmentVariable{}, ErrVariableNotFound
	}
	v.EncryptedKey = req.EncryptedKey
	v.EncryptedValue = req.EncryptedValue
	if err := s.db.Save(&v).Error; err != nil {
		return models.EnvironmentVariable{}, err
	}
	return v, nil
}

func (s *VariableService) Delete(variableID uint) error {
	res := s.db.Delete(&models.EnvironmentVariable{}, variableID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrVariableNotFound
	}
	return nil
}
