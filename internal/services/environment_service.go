package services

import (
	"strings"

	"curtain/internal/dto"
	"curtain/internal/models"

	"gorm.io/gorm"
)

type EnvironmentService struct {
	db *gorm.DB
}

func NewEnvironmentService(db *gorm.DB) *EnvironmentService {
	return &EnvironmentService{db: db}
}

func (s *EnvironmentService) Create(projectID uint, req dto.CreateEnvironmentRequest) (models.Environment, error) {
	env := models.Environment{ProjectID: projectID, Name: req.Name}
	if err := s.db.Create(&env).Error; err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
			return models.Environment{}, ErrEnvironmentTaken
		}
		return models.Environment{}, err
	}
	return env, nil
}

func (s *EnvironmentService) ListByProject(projectID uint) ([]models.Environment, error) {
	var envs []models.Environment
	if err := s.db.Where("project_id = ?", projectID).Order("id ASC").Find(&envs).Error; err != nil {
		return nil, err
	}
	if envs == nil {
		envs = []models.Environment{}
	}
	return envs, nil
}

func (s *EnvironmentService) Delete(envID uint) error {
	return s.db.Delete(&models.Environment{}, envID).Error
}
