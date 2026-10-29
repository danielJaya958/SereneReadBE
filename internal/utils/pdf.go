package utils

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func GenerateThumbnail(pdfFullPath string, outputDir string, outputFileName string) (string, error) {
	// Buang ekstensi .pdf dari nama output, karena pdftoppm akan otomatis tambah .jpg
	baseName := outputFileName[:len(outputFileName)-len(filepath.Ext(outputFileName))]

	// Command: pdftoppm -f 1 -l 1 -singlefile -jpeg input.pdf output_dir/baseName
	cmd := exec.Command("pdftoppm", "-f", "1", "-l", "1", "-singlefile", "-jpeg", pdfFullPath, outputDir+"/"+baseName)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gagal membuat thumbnail: %v", err)
	}

	return baseName + ".jpg", nil
}
