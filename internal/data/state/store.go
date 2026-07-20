package state

import db "ardents/internal/persistence"

func PathInDir(dir string) string {
	return db.PathInDir(dir)
}

func LoadSnapshot(path string, out any) (bool, error) {
	return db.LoadJSON(path, "data", "snapshot", out)
}

func SaveSnapshot(path string, snapshot any) error {
	return db.SaveJSON(path, "data", "snapshot", snapshot)
}
