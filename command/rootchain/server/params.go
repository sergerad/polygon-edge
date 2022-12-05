package server

const (
	dataDirFlag = "data-dir"
	premineFlag = "premine"
)

type serverParams struct {
	dataDir string
	premine []string
}
