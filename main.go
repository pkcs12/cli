package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/beevik/etree"
	"golang.org/x/term"
)

type Config struct {
	TIN     string `json:"TIN"`
	Name    string `json:"Name"`
	VAT     string `json:"VAT"`
	Address string `json:"Address"`
	Town    string `json:"Town"`
	Country string `json:"Country"`

	Environment   string `json:"Environment"`
	BusinUnitCode string `json:"BusinUnitCode"`
	TCRCode       string `json:"TCRCode"`
	SoftCode      string `json:"SoftCode"`
	OperatorCode  string `json:"OperatorCode"`
	Typless       struct {
		APIKey   string `json:"APIKey"`
		Template string `json:"Template"`
	} `json:"Typless"`
}

var (
	workDir     = ""
	extractExec = "./extract"
	bridgeExec  = "./bridge"
	iicExec     = "./iic"
	dsigExec    = "./dsig"
	regExec     = "./reg"
	keepExec    = "./keep"
	qrcExec     = "./qrc"
	pdfExec     = "./pdf"
)

// FatalIfNoValue returns a value is no erro, exits otherwise
func FatalIfNoValue(value interface{}, err error) interface{} {
	if err != nil {
		log.Fatal(err)
	}
	return value
}

// FatalIfErr returns a value is no erro, exits otherwise
func FatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func exitOnError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func valueOrExitOnError(any interface{}, err error) interface{} {
	exitOnError(err)
	return any
}

func validateConfig(cfg *Config) error {
	if cfg.TIN == "" || cfg.Name == "" || cfg.VAT == "" || cfg.BusinUnitCode == "" || cfg.TCRCode == "" || cfg.SoftCode == "" || cfg.OperatorCode == "" {
		return errors.New("Invalid config")
	}
	return nil
}

// Scan prompts for input and returns given value
func Scan(msg string) string {
	fmt.Print(msg)
	var value string

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	value = scanner.Text()
	return value
}

func main() {
	if runtime.GOOS == "windows" {
		extractExec = strings.Join([]string{extractExec, "exe"}, ".")
		bridgeExec = strings.Join([]string{bridgeExec, "exe"}, ".")
		iicExec = strings.Join([]string{iicExec, "exe"}, ".")
		dsigExec = strings.Join([]string{dsigExec, "exe"}, ".")
		regExec = strings.Join([]string{regExec, "exe"}, ".")
		keepExec = strings.Join([]string{keepExec, "exe"}, ".")
		qrcExec = strings.Join([]string{qrcExec, "exe"}, ".")
		pdfExec = strings.Join([]string{pdfExec, "exe"}, ".")
	}
	fmt.Println("EFI Command Line Interface")
	fmt.Println("--------------------------")
	fmt.Println("Available actions:")
	fmt.Println("[1] Register invoice")
	fmt.Println("[2] Register invoice manually")

	fmt.Println("Press any other kay to exit")

	fmt.Printf("Choose an action: ")
	action := 0
	stringValue := Scan("Choose an action: ")
	action = FatalIfNoValue(strconv.Atoi(stringValue)).(int)
	switch action {
	case 1:
		registerInvoice()
	case 2:
		registerInvoiceManually()
	}
}

func registerInvoice() {

	// REQUIREMENTS
	// working dir
	workDir := valueOrExitOnError(filepath.Abs(filepath.Dir(os.Args[0]))).(string)
	os.Chdir(workDir)
	fmt.Printf("WorkDir: %s\n", workDir)

	// Config
	cfgFilePath := path.Join(workDir, "config.json")
	buf := valueOrExitOnError(ioutil.ReadFile(cfgFilePath)).([]byte)
	cfg := Config{}
	exitOnError(json.Unmarshal(buf, &cfg))
	exitOnError(validateConfig(&cfg))

	// PIN
	fmt.Print("Please provide digital signature PIN: ")
	buf = valueOrExitOnError(term.ReadPassword(int(os.Stdin.Fd()))).([]byte)
	pin := string(buf)
	fmt.Println()

	// Invoice file
	fmt.Print("Please provide invoice file path: ")
	extractInFile := Scan("Please provide invoice file path: ")

	fmt.Println(extractInFile)
	if fi := valueOrExitOnError(os.Stat(extractInFile)).(os.FileInfo); fi.IsDir() {
		fmt.Fprintf(os.Stderr, errors.New("Not a file").Error())
		os.Exit(1)
	}

	SkipExtract := false

	// EXTRACT
	_, fileNameWithExt := filepath.Split(extractInFile)
	fileNameNoExt := strings.TrimRight(fileNameWithExt, filepath.Ext(fileNameWithExt))
	fileName := strings.Join([]string{fileNameNoExt, "extract"}, ".")
	extractOutFile := path.Join(workDir, fileName)
	if !SkipExtract {
		cmd := exec.Command(
			extractExec,
			"-key", cfg.Typless.APIKey,
			"-template", cfg.Typless.Template,
			"-in", extractInFile,
			"-out", extractOutFile,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Print("Extracting: ")
		exitOnError(cmd.Run())
		fmt.Println("OK")
	}

	// BRIDGE
	fileName = strings.Join([]string{fileNameNoExt, "bridge"}, ".")
	bridgeOutFile := path.Join(workDir, fileName)
	cmd := exec.Command(
		bridgeExec,
		"-soft", cfg.SoftCode,
		"-op", cfg.OperatorCode,
		"-busin", cfg.BusinUnitCode,
		"-tcr", cfg.TCRCode,
		"-in", extractOutFile,
		"-out", bridgeOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("Bridging: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// IIC
	fileName = strings.Join([]string{fileNameNoExt, "iic"}, ".")
	iicOutFile := path.Join(workDir, fileName)
	cmd = exec.Command(
		iicExec,
		"-pin", pin,
		"-in", bridgeOutFile,
		"-out", iicOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("IIC: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// DSIG
	fileName = strings.Join([]string{fileNameNoExt, "dsig"}, ".")
	dsigOutFile := path.Join(workDir, fileName)
	cmd = exec.Command(
		dsigExec,
		"-pin", pin,
		"-busin", cfg.BusinUnitCode,
		"-soft", cfg.SoftCode,
		"-in", iicOutFile,
		"-out", dsigOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("DSIG: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// REG
	fileName = strings.Join([]string{fileNameNoExt, "reg"}, ".")
	regOutFile := path.Join(workDir, fileName)
	cmd = exec.Command(
		regExec,
		"-pin", pin,
		"-env", cfg.Environment,
		"-in", dsigOutFile,
		"-out", regOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("REG: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// KEEP
	cmd = exec.Command(
		keepExec,
		"-req", dsigOutFile,
		"-resp", regOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("KEEP: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// QRC
	fileName = strings.Join([]string{fileNameNoExt, "qrc"}, ".")
	qrcOutFile := path.Join(workDir, fileName)
	cmd = exec.Command(
		qrcExec,
		"-req", dsigOutFile,
		"-resp", regOutFile,
		"-out", qrcOutFile,
		"-env", cfg.Environment,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("QRC: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// PDF
	fileName = strings.Join([]string{fileNameNoExt, "reg", "pdf"}, ".")
	pdfOutFile := path.Join(workDir, fileName)
	cmd = exec.Command(
		pdfExec,
		"-in", extractInFile,
		"-out", pdfOutFile,
		"-req", dsigOutFile,
		"-resp", regOutFile,
		"-qr", qrcOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("PDF: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	fmt.Printf("Invoice registered. New PDF file: %s\n\nEnjoy your day\nPres enter for exit\n", pdfOutFile)
	fmt.Scan()
}

func registerInvoiceManually() {

	// REQUIREMENTS
	// working dir
	workDir := valueOrExitOnError(filepath.Abs(filepath.Dir(os.Args[0]))).(string)
	os.Chdir(workDir)
	fmt.Printf("WorkDir: %s\n", workDir)

	// Config
	cfgFilePath := path.Join(workDir, "config.json")
	buf := valueOrExitOnError(ioutil.ReadFile(cfgFilePath)).([]byte)
	cfg := Config{}
	exitOnError(json.Unmarshal(buf, &cfg))
	exitOnError(validateConfig(&cfg))

	// PIN
	fmt.Print("Please provide digital signature PIN: ")
	buf = FatalIfNoValue(term.ReadPassword(int(os.Stdin.Fd()))).([]byte)
	pin := string(buf)
	fmt.Println()

	// Invoice file
	bridgeInFile := Scan("Please provide extracted invoice file path(.bridge): ")
	if fi := valueOrExitOnError(os.Stat(bridgeInFile)).(os.FileInfo); fi.IsDir() {
		fmt.Fprintf(os.Stderr, errors.New("Not a file").Error())
		os.Exit(1)
	}

	// EXTRACT
	_, fileNameWithExt := filepath.Split(bridgeInFile)
	fileNameNoExt := strings.TrimRight(fileNameWithExt, filepath.Ext(fileNameWithExt))
	fileName := strings.Join([]string{fileNameNoExt, "bridge"}, ".")
	bridgeOutFile := path.Join(workDir, fileName)

	// IIC
	fileName = strings.Join([]string{fileNameNoExt, "iic"}, ".")
	iicOutFile := path.Join(workDir, fileName)
	cmd := exec.Command(
		iicExec,
		"-pin", pin,
		"-in", bridgeOutFile,
		"-out", iicOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("IIC: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// DSIG
	fileName = strings.Join([]string{fileNameNoExt, "dsig"}, ".")
	dsigOutFile := path.Join(workDir, fileName)
	cmd = exec.Command(
		dsigExec,
		"-pin", pin,
		"-busin", cfg.BusinUnitCode,
		"-soft", cfg.SoftCode,
		"-in", iicOutFile,
		"-out", dsigOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("DSIG: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// REG
	fileName = strings.Join([]string{fileNameNoExt, "reg"}, ".")
	regOutFile := path.Join(workDir, fileName)
	cmd = exec.Command(
		regExec,
		"-pin", pin,
		"-env", cfg.Environment,
		"-in", dsigOutFile,
		"-out", regOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("REG: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// KEEP
	cmd = exec.Command(
		keepExec,
		"-req", dsigOutFile,
		"-resp", regOutFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("KEEP: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	// QRC
	fileName = strings.Join([]string{fileNameNoExt, "qrc"}, ".")
	qrcOutFile := path.Join(workDir, fileName)
	cmd = exec.Command(
		qrcExec,
		"-req", dsigOutFile,
		"-resp", regOutFile,
		"-out", qrcOutFile,
		"-env", cfg.Environment,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Print("QRC: ")
	exitOnError(cmd.Run())
	fmt.Println("OK")

	doc := etree.NewDocument()
	exitOnError(doc.ReadFromFile(dsigOutFile))

	invoice := doc.FindElement("//Invoice")
	if invoice == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no Invoice"))
		os.Exit(1)
	}
	attr := invoice.SelectAttr("IIC")
	if attr == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no IIC"))
		os.Exit(1)
	}
	IIC := attr.Value

	doc = etree.NewDocument()
	exitOnError(doc.ReadFromFile(regOutFile))

	fic := doc.FindElement("//FIC")
	if invoice == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no FIC"))
		os.Exit(1)
	}
	FIC := fic.Text()

	fmt.Println("Invoice registered")
	fmt.Fprintf(os.Stdout, "IKOF (Kôd izdavaoca računa): %v\n", IIC)
	fmt.Fprintf(os.Stdout, "JIKR (Jedinstveni identifikacioni kod računa): %v\n", FIC)
	fmt.Fprintf(os.Stdout, "QR Code saved at: %v\n", qrcOutFile)

	fmt.Print("Pres enter for exit")
	fmt.Scan()
}
