package repo

import (
	"errors"
	"rip/internal/pkg/repo"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

// TODO: можно ли добавить услугу после формирования???
// PS: ща нельзя

func (r *Repository) CreateEncryptDecryptDraft(creatorID uint) (uint, error) {
	request := repo.EncryptDecryptRequest{
		CreatorID:    creatorID,
		Status:       repo.Draft,
		CreationDate: r.db.NowFunc(),
	}
	if err := r.db.Clauses(clause.Returning{}).Select("request_id").Create(&request).Error; err != nil {
		return 0, err
	}
	return request.RequestID, nil
}

func (r *Repository) AddDataServiceToDraft(dataID uint, creatorID uint) (uint, error) {
	// получаем услугу
	data, err := r.GetDataServiceById(dataID)
	if err != nil {
		return 0, err
	}

	if data == nil {
		return 0, errors.New("нет такой услуги")
	}
	if data.Active {
		return 0, errors.New("услуга удалена")
	}

	// получаем черновик
	var draftReq repo.EncryptDecryptRequest
	res := r.db.Where("creator_id = ?", creatorID).Where("status = ?", repo.Draft).Take(&draftReq)

	// создаем черновик, если его нет
	if res.RowsAffected == 0 {
		newDraftRequestID, err := r.CreateEncryptDecryptDraft(creatorID)
		if err != nil {
			return 0, err
		}

		draftReq.RequestID = newDraftRequestID
	}

	// добавляем запись в мм
	requestToData := repo.EncryptDecryptToData{
		DataID:    dataID,
		RequestID: draftReq.RequestID,
	}

	err = r.db.Create(&requestToData).Error
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return 0, errors.New("услуга уже существует в заявке")
		}

		return 0, err
	}

	return draftReq.RequestID, nil
}

func (r *Repository) DeleteDataServiceFromDraft(dataID uint, creatorID uint) error {
	// получаем услугу
	data, err := r.GetDataServiceById(dataID)
	if err != nil {
		return err
	}

	if data == nil {
		return errors.New("нет такой услуги")
	}
	if data.Active {
		return errors.New("услуга удалена")
	}

	// получаем черновик
	draftRequestID, err := r.GetEncryptDecryptDraftID(creatorID)
	if err != nil {
		return err
	}
	if draftRequestID == nil {
		return errors.New("у пользователя нет черновика-заявки")
	}

	// удаляем услугу из черновика
	var requestToData repo.EncryptDecryptToData

	// TODO: если не нашли??
	if err := r.db.Delete(&requestToData).Error; err != nil {
		return err
	}

	return nil
}

// returns nil if there is no draft
func (r *Repository) GetEncryptDecryptDraftID(creatorID uint) (*uint, error) {
	var draftReq repo.EncryptDecryptRequest
	res := r.db.Where("creator_id = ?", creatorID).Where("status = ?", repo.Draft).Take(&draftReq)

	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &draftReq.RequestID, nil
}

func (r *Repository) GetEncryptDecryptRequests(status repo.Status, startDate, endDate time.Time) ([]repo.EncryptDecryptRequest, error) {
	var requests []repo.EncryptDecryptRequest

	filterCond := r.db.Where("status = ?", status).Where("form_date > ?", startDate)
	if !endDate.IsZero() {
		filterCond = filterCond.Where("form_date < ?", endDate)
	}

	if err := filterCond.Find(&requests).Error; err != nil {
		return nil, err
	}

	return requests, nil
}

func (r *Repository) GetEncryptDecryptRequestWithDataByID(requestID uint) (repo.EncryptDecryptRequest, []repo.DataService, error) {
	request := repo.EncryptDecryptRequest{RequestID: requestID}
	if err := r.db.First(&request); err != nil {
		return repo.EncryptDecryptRequest{}, nil, nil
	}

	var dataService []repo.DataService
	// TODO: test
	res := r.db.
		Table("encrypt_decrypt_to_data").
		Where("request_id = ?", requestID).
		Joins("JOIN data_service d on d.data_id = encrypt_decrypt_to_data.data_id").
		Where("active = ?", true).
		Find(&dataService)

	if err := res.Error; err != nil {
		return repo.EncryptDecryptRequest{}, nil, nil
	}

	return request, dataService, nil
}

// creator
func (r *Repository) FormEncryptDecryptRequestByID(requestID uint) error {
	var req repo.EncryptDecryptRequest
	res := r.db.
		Where("request_id = ?", requestID).
		Where("status = ?", repo.Draft).
		Take(&req)

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("нет такой заявки")
	}

	req.Status = repo.Formed
	req.FormDate = r.db.NowFunc() // наверно не прокнет тк это алиас к time.Now().Local()

	if err := r.db.Save(&req).Error; err != nil {
		return err
	}

	return nil
}

func (r *Repository) DeleteEncryptDecryptRequestByID(requestID uint) error {
	var req repo.EncryptDecryptRequest
	res := r.db.
		Where("request_id = ?", requestID).
		Where("status in (?)", []repo.Status{repo.Draft, repo.Formed}).
		Take(&req)

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("нет такой заявки")
	}

	req.Status = repo.Deleted
	// надо ли ставить finish date

	if err := r.db.Save(&req).Error; err != nil {
		return err
	}

	return nil
}

// moderator
func (r *Repository) FinishEncryptDecryptRequestByID(requestID, moderatorID uint) error {
	return r.finishRejectHelper(repo.Finished, requestID, moderatorID)
}

func (r *Repository) RejectEncryptDecryptRequestByID(requestID, moderatorID uint) error {
	return r.finishRejectHelper(repo.Rejected, requestID, moderatorID)
}

func (r *Repository) finishRejectHelper(status repo.Status, requestID, moderatorID uint) error {
	var req repo.EncryptDecryptRequest
	res := r.db.
		Where("request_id = ?", requestID).
		Where("status = ?", repo.Draft).
		Take(&req)

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("нет такой заявки")
	}

	req.ModeratorID = moderatorID
	req.Status = status
	/*if req.Status == repo.Finished {
		TODO: добавить результат в мм
	}*/
	req.FinishDate = r.db.NowFunc() // наверно не прокнет тк это алиас к time.Now().Local()

	if err := r.db.Save(&req).Error; err != nil {
		return err
	}

	return nil
}