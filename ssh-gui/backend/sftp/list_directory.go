package sftp

import gosftp "github.com/pkg/sftp"

func ListDirectory(
	client *gosftp.Client,
	path string,
) (
	[]FileItem,
	error,
) {

	files, err :=
		client.ReadDir(path)

	if err != nil {
		return nil, err
	}

	result :=
		[]FileItem{}

	for _, file := range files {

		result =
			append(
				result,
				FileItem{
					Name: file.Name(),
					Path: path + "/" + file.Name(),

					IsDirectory:
						file.IsDir(),

					Size:
						file.Size(),

					Mode:
						file.Mode().String(),
				},
			)

	}

	return result, nil
}
