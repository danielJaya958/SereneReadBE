package utils

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

// GetPdfPageCount membaca jumlah halaman PDF menggunakan pdfinfo (bagian dari Poppler)
func GetPdfPageCount(pdfFullPath string) (int, error) {
	out, err := exec.Command("pdfinfo", pdfFullPath).Output()
	if err != nil {
		return 0, fmt.Errorf("gagal membaca info PDF: %v", err)
	}

	// Cari baris "Pages:          123"
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			return strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Pages:")))
		}
	}

	return 0, errors.New("jumlah halaman tidak ditemukan di PDF")
}
