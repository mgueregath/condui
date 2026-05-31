package storage

import "ssh-gui/backend/models"

func (d *Database) GetFolders() (
	[]models.Folder,
	error,
) {

	rows, err :=
		d.DB.Query(
			`
			SELECT
				id,
				name,
				parent_id
			FROM folders
			ORDER BY name
			`,
		)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result :=
		[]models.Folder{}

	for rows.Next() {

		var folder models.Folder

		err := rows.Scan(
			&folder.ID,
			&folder.Name,
			&folder.ParentID,
		)

		if err != nil {
			return nil, err
		}

		result =
			append(
				result,
				folder,
			)
	}

	return result, nil
}
