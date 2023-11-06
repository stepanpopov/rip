package repo

import (
	"rip/internal/pkg/repo"
	"strings"
)

// nil если такой заявки нет
func (r *Repository) GetDataServiceById(id uint) (*repo.DataService, error) {
	dataService := repo.DataService{DataID: id}
	res := r.db.Take(&dataService)
	if res.Error != nil {
		return nil, res.Error
	}

	if res.RowsAffected == 0 {
		return nil, nil
	}

	return &dataService, nil
}

func (r *Repository) GetActiveDataServiceFilteredByName(name string) ([]repo.DataService, error) {
	name = strings.ToLower(name)

	var dataService []repo.DataService
	if err := r.db.Where(&repo.DataService{Active: true}).Where("LOWER(data_name) LIKE ?", "%"+name+"%").Find(&dataService).Error; err != nil {
		return nil, err
	}

	return dataService, nil
}

func (r *Repository) UpdateDataService(d *repo.DataService) error {
	err := r.db.Model(&d).Where("active = ?", true).Updates(map[string]interface{}{"data_name": d.DataName, "encode": d.Encode, "blob": d.Blob}).Error
	return err
}

func (r *Repository) CreateDataService(d repo.DataService) error {
	d.Active = true
	return r.db.Create(d).Error
}

func (r *Repository) DeleteDataService(id uint) error {
	if err := r.db.Exec("UPDATE data_services SET active = false WHERE data_id = ?", id).Error; err != nil {
		return err
	}
	return nil
}

/* TODO
func (r *Repository) UpdateImage(dataID uint) error {

} */