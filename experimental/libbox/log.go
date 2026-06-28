//go:build darwin || linux || windows

package libbox

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"filippo.io/age"
)

type crashReportMetadata struct {
	reportMetadata
	CrashedAt       string `json:"crashedAt,omitempty"`
	SignalName      string `json:"signalName,omitempty"`
	SignalCode      string `json:"signalCode,omitempty"`
	ExceptionName   string `json:"exceptionName,omitempty"`
	ExceptionReason string `json:"exceptionReason,omitempty"`
}

func archiveCrashReport(path string, crashReportsDir string) {
	content, err := os.ReadFile(path)
	if err != nil || len(content) == 0 {
		return
	}

	info, _ := os.Stat(path)
	crashTime := time.Now().UTC()
	if info != nil {
		crashTime = info.ModTime().UTC()
	}

	initReportDir(crashReportsDir)
	destPath, err := nextAvailableReportPath(crashReportsDir, crashTime)
	if err != nil {
		return
	}
	initReportDir(destPath)

	writeReportFile(destPath, "go.log", content)
	metadata := crashReportMetadata{
		reportMetadata: baseReportMetadata(),
		CrashedAt:      crashTime.Format(time.RFC3339),
	}
	writeReportMetadata(destPath, metadata)
	os.Remove(path)
	copyConfigSnapshot(destPath)
}

func configSnapshotPath() string {
	return filepath.Join(sBasePath, "configuration.json")
}

func saveConfigSnapshot(configContent string) {
	snapshotPath := configSnapshotPath()
	os.WriteFile(snapshotPath, []byte(configContent), 0o666)
	chownReport(snapshotPath)
}

func redirectStderr(path string) error {
	crashReportsDir := filepath.Join(sWorkingPath, "crash_reports")
	archiveCrashReport(path, crashReportsDir)
	archiveCrashReport(path+".old", crashReportsDir)

	outputFile, err := os.Create(path)
	if err != nil {
		return err
	}
	if runtime.GOOS != "android" && runtime.GOOS != "windows" {
		err = outputFile.Chown(sUserID, sGroupID)
		if err != nil {
			outputFile.Close()
			os.Remove(outputFile.Name())
			return err
		}
	}

	err = debug.SetCrashOutput(outputFile, debug.CrashOptions{})
	if err != nil {
		outputFile.Close()
		os.Remove(outputFile.Name())
		return err
	}
	_ = outputFile.Close()
	return nil
}

func CreateZipArchive(sourcePath string, destinationPath string, encrypt bool) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return os.ErrInvalid
	}

	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = destinationFile.Close()
	}()

	var archiveWriter io.Writer = destinationFile
	var encryptedWriter io.WriteCloser
	if encrypt {
		const reportEncryptionRecipient = "age1pq18zn32skjt2ns56lja2nl4sztmx5u820fp67z2dg6ueyffy5l30gkdlae58a0qfd0ms5729gsdenkxdd2s55eh9g6nx80s9e757jf853p5r48vs2wkzyqyde5qd2hqpv9f5sfejn2svyqxse22pzz2l5vee4g90j0d2jsmycgwv4v4y79dshdc8jwjvkuhsawt7s22vgccdww2fpfxjke9gje4ugkgwn8938ejhh6y63u0uf5x0gxn706x0j00nwkesy3w3yryqun23f2e2zxxfv85wnwtaf5rsu4arrkhk6jsazqcjtps8qv0sz3z966hyspxyq7zqsgntyvhzqxpal6ex6khg267fpe5lxzknugynw48fsmqugncysxgpyprljmnce6dehtv6mu2v3x2qvudlkvhj89s58u3qwu5cerqfam9kdmtyd64y68p0mwclrmpume3gz47nkyjnj6c30elz3x4fv926qn3ulz8kmnwkxu4yg08v2as86yukzgfvgq8fxjchy2lkzrtr9jdyp5ymk7c7wn69jgcays3t3x5dlswmuphr4fqctj96zgftuqs0v95744h2h7gwdpn2nmtz26gtrsq3dcwp7l2qpldkcg64zx5k5g37zq4fspn2xesvmftvd2cwcgf93l0dcmwyz7zkfymdq6a6fyp2u738hx92cc6k4kejn5zcqgslw60g6ejueg8kyn4feycklg4v7utfsd5x3f8pc63xnj2la9wzlgn9992j3spq4c6l4s9s594gfzrps46pakwwspcx2njleu8xm9ys059f9zwfma0je40qlj8czsgx0pqkyfhxewclphty82q5s7rsyvya39a753eez5y85s9arn8vrftfsdzpvz44guus2qxzzl8wuk8fw0ykmj7vyq4wqzy5uwvxaywxf3f94jeh38lm66tvrzc842zx3p59x9dxmmt78prqftngjn6f5v2kvcphc6nspv8nfa5xcvpw9apft2583qux0tnvfwht8tkagymv2z84pccm8mdke9d9pw0ymhp9pnnqeyndwp8hzyjdcfr2phwxktawn5umav8zyw3v9wcsgaq9c36l52wd4s3uf3jsxh2dukw6vqh2t5ww9crl2mvv2zsua7q5v35l4mpkzn45ps9w6mewfd0q6ar0fhfd0eevvseyel5n8az3s5xnv627u2ggq4jkw75mzg5wxpadzg32668ts04xd9n92g6ct3ewkfwku80fevy7ypj6qxddn6fanztvngw499syynu9d6ak3070uq7wg3frc3c7jphz49vxfhe4fh2ggkfjsup2reveyvwcqp9j9nzfj6gysmc3tu8wux35p2tvd6mysu2sleslrds7zyewj9kj4qzqvy4juj0yn0pvhljfxr92acus8ztzvfsv5ekx92rywzqhtg9rav074esg9rqzsenpzjv0r87slhczwrkdt22gka0w4hnrj27rpp0rmw2z7dgnz6ly2p0fqh845ej4jujkk86kz4pcdld73rez2crqe65gc8437l4kt40v2e9lxe8raqxvr5wx9m44jpc5nr4ydhh7mtv0aqkx3e7w96jep2l63ez3wecaja2g47z7f9dyd6hpgfsejx25y3mpfsx4uncmmm5uvg9dhyy0lad2wfg2ju6jrqa5d5k6zgvcmy7l8pfasfj7yt0z5qqap5yvc4av4vhjq86ff6sfqj5x0lxzug5ca08yua72pgjgx49y0ftq63u3v53wmy2t9jg77sfwjvzcka5ur6z922gua6g323dtxggwkkqq7z6uu9mpkrjre2nd0lw04x4gt4gcn4llm4wh6ksvu7nx4erttv47fgzcw9sdxx0c6ll9pf0ntae5clp3qclwg4uhvd02lenv0mard9lmtalss2sg8d4ay78u4cgem02swh3pq5"
		var recipient *age.HybridRecipient
		recipient, err = age.ParseHybridRecipient(reportEncryptionRecipient)
		if err != nil {
			return err
		}
		encryptedWriter, err = age.Encrypt(destinationFile, recipient)
		if err != nil {
			return err
		}
		archiveWriter = encryptedWriter
	}

	zipWriter := zip.NewWriter(archiveWriter)

	rootName := filepath.Base(sourcePath)
	err = filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}

		archivePath := filepath.ToSlash(filepath.Join(rootName, relativePath))
		if d.IsDir() {
			_, err = zipWriter.Create(archivePath + "/")
			return err
		}

		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}
		header.Name = archivePath
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, sourceFile)
		closeErr := sourceFile.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		_ = zipWriter.Close()
		if encryptedWriter != nil {
			_ = encryptedWriter.Close()
		}
		return err
	}

	err = zipWriter.Close()
	if err != nil {
		if encryptedWriter != nil {
			_ = encryptedWriter.Close()
		}
		return err
	}
	if encryptedWriter != nil {
		return encryptedWriter.Close()
	}
	return nil
}
