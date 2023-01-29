package packagemanager

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/gopherclass/go-shellquote"
	"github.com/pkg/errors"
)

type pack struct {
	DownloadPathes []string
	LookupPath     []string
	InstallCommand string
}

var repository = map[string]pack{
	NginxPackage: {
		LookupPath: []string{"nginx"},
		DownloadPathes: []string{
			"http://nginx.org/download/nginx-1.22.1.zip",
		},
	},
	MariaDBServerPackage: {
		LookupPath: []string{"mysql", "mariadb"},
		DownloadPathes: []string{
			"https://mirror.23m.com/mariadb/mariadb-10.6.11/winx64-packages/mariadb-10.6.11-winx64.msi",
			"https://ftp.bme.hu/pub/mirrors/mariadb/mariadb-10.6.11/winx64-packages/mariadb-10.6.11-winx64.msi",
		},
		InstallCommand: "msiexec /i mariadb-10.6.11-winx64.msi SERVICENAME=MariaDB PORT=3306 /qn",
	},
	PHPPackage: {
		LookupPath: []string{"php"},
		DownloadPathes: []string{
			"https://windows.php.net/downloads/releases/php-8.2.1-Win32-vs16-x64.zip",
		},
	},
}

type WindowsPackageManager struct {
}

func NewWindowsPackageManager() *WindowsPackageManager {
	return &WindowsPackageManager{}
}

func (pm *WindowsPackageManager) Search(_ context.Context, _ string) ([]PackageInfo, error) {
	return nil, nil
}

func (pm *WindowsPackageManager) Install(ctx context.Context, packs ...string) error {
	var err error
	for _, p := range packs {
		repoPack, exists := repository[p]
		if !exists {
			continue
		}

		err = pm.installPackage(ctx, p, repoPack)
		if err != nil {
			return err
		}
	}

	return nil
}

func (pm *WindowsPackageManager) installPackage(ctx context.Context, packName string, p pack) error {
	var err error

	packagePath := ""
	for _, c := range p.LookupPath {
		packagePath, err = exec.LookPath(c)
		if err != nil {
			continue
		}

		log.Printf("Package %s is found in path '%s'\n", packName, filepath.Dir(packagePath))
		break
	}

	processor, ok := packageProcessors[packName]
	if ok {
		err = processor(ctx, packagePath)
		if err != nil {
			return err
		}
	}

	if packagePath != "" {
		return nil
	}

	dir, err := os.MkdirTemp("", "install")
	if err != nil {
		return errors.WithMessagef(err, "failed to make temp directory")
	}

	for _, path := range p.DownloadPathes {
		err = utils.DownloadFile(ctx, path, dir)
		if err != nil {
			log.Println("failed to download file")
			log.Println(err)
			continue
		}

		break
	}

	if p.InstallCommand == "" {
		return errors.New("empty install command for package")
	}

	cmd, err := shellquote.Split(p.InstallCommand)
	if err != nil {
		return errors.WithMessage(err, "failed to split command")
	}

	return utils.ExecCommand(cmd[0], cmd[1:]...)
}

func (pm *WindowsPackageManager) CheckForUpdates(_ context.Context) error {
	return nil
}

func (pm *WindowsPackageManager) Remove(_ context.Context, _ ...string) error {
	return errors.New("removing packages is not supported on Windows")
}

func (pm *WindowsPackageManager) Purge(_ context.Context, _ ...string) error {
	return errors.New("removing packages is not supported on Windows")
}

var packageProcessors = map[string]func(ctx context.Context, packagePath string) error{
	PHPExtensionsPackage: func(ctx context.Context, packagePath string) error {
		cmd := exec.Command("php", "-r", "echo php_ini_scanned_files();")
		buf := &bytes.Buffer{}
		buf.Grow(100)
		cmd.Stdout = buf
		cmd.Stderr = log.Writer()
		log.Println("\n", cmd.String())
		err := cmd.Run()
		if err != nil {
			return errors.WithMessage(err, "failed to get scanned files")
		}

		scannedFiles := strings.Split(buf.String(), "\n")

		if len(scannedFiles) > 0 {
			firstScannedFile := strings.TrimSpace(scannedFiles[0])
			scannedFileDir := filepath.Dir(firstScannedFile)

			exts := []string{
				"bz2", "curl", "fileinfo", "gd", "gmp", "intl",
				"mbstring", "openssl", "pdo_mysql", "pdo_sqlite", "zip",
			}

			for _, e := range exts {
				err = utils.WriteContentsToFile([]byte(`extension=`+e), filepath.Join(scannedFileDir, e+".ini"))
				if err != nil {
					return errors.WithMessagef(err, "failed to create ini for '%s' php extension", e)
				}
			}
		}

		cmd = exec.Command("php", "-r", "echo php_ini_loaded_file();")
		buf = &bytes.Buffer{}
		buf.Grow(100)
		cmd.Stdout = buf
		cmd.Stderr = log.Writer()
		log.Println("\n", cmd.String())
		err = cmd.Run()
		if err != nil {
			return errors.WithMessage(err, "failed to get ini loaded file from php")
		}
		loadedFiles := strings.Split(buf.String(), "\n")
		iniFilePath := ""
		if len(loadedFiles) > 0 {
			iniFilePath = strings.TrimSpace(loadedFiles[0])
		}
		if iniFilePath == "" {
			iniFilePath = filepath.Join(filepath.Dir(packagePath), "php.ini")
		}
		if iniFilePath != "" {
			return utils.FindLineAndReplaceOrAdd(ctx, iniFilePath, map[string]string{
				";?\\s*extension=bz2\\s*":        "extension=bz2",
				";?\\s*extension=curl\\s*":       "extension=curl",
				";?\\s*extension=fileinfo\\s*":   "extension=fileinfo",
				";?\\s*extension=gd\\s*":         "extension=gd",
				";?\\s*extension=gmp\\s*":        "extension=gmp",
				";?\\s*extension=intl\\s*":       "extension=intl",
				";?\\s*extension=mbstring\\s*":   "extension=mbstring",
				";?\\s*extension=openssl\\s*":    "extension=openssl",
				";?\\s*extension=pdo_mysql\\s*":  "extension=pdo_mysql",
				";?\\s*extension=pdo_sqlite\\s*": "extension=pdo_sqlite",
				";?\\s*extension=zip\\s*":        "extension=zip",
			})
		}

		return errors.New("failed to find config edition way to enable php extensions")
	},
}
