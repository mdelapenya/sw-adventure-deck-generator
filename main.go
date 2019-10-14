package main

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/flopp/go-findfont"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/manifoldco/promptui"
)

type imageDimensions struct {
	x int
	y int
}

var basePath = "."

const defaultCardsPath = "cards"
const defaultImagesPath = "images"
const defaultBodyFont = "Arial"
const defaultHeaderFont = "Adventure"
const defaultTitleFont = "Adventure"
const defaultOutputsPath = "outputs"
const defaultTemplatesPath = "templates"
const defaultTextsPath = "texts"

var cardsDims = imageDimensions{
	x: 243, y: 340,
}
var imageDims = imageDimensions{
	x: 196, y: 157,
}

type cardConfig struct {
	BodyFont    string
	HeaderFont  string
	ImagesPath  string
	OutputsPath string
	Template    string
	TextsPath   string
	TitleFont   string
}

func main() {
	basePath = promptBasePath()

	imagesPath := promptPath("images directory", path.Join(basePath, defaultImagesPath))
	validateImages(imagesPath, imageDims)

	templatesPath := promptPath("templates directory", path.Join(basePath, defaultTemplatesPath))
	validateImages(templatesPath, cardsDims)
	template := promptTemplate(templatesPath)

	textsPath := promptPath("texts directory", path.Join(basePath, defaultTextsPath))
	outputsPath := promptPath("output directory", path.Join(basePath, defaultOutputsPath))

	hf := promptTrueTypeFonts("Header")
	bf := promptTrueTypeFonts("Title and Body")

	cardCfg := cardConfig{
		HeaderFont:  hf,
		TitleFont:   bf,
		BodyFont:    bf,
		Template:    path.Join(templatesPath, template),
		OutputsPath: outputsPath,
		ImagesPath:  imagesPath,
		TextsPath:   textsPath,
	}

	processImages(cardCfg)
	validateImages(outputsPath, cardsDims)
}

type textConfig struct {
	label    string
	font     string
	fontSize float64
	spacing  float64
	x        float64
	y        float64
	ax       float64
	ay       float64
	width    float64
	output   string
}

func addText(source image.Image, cfg textConfig) {
	_, fontPath := getTrueTypeFont(cfg.font)

	x := source.Bounds().Max.X
	y := source.Bounds().Max.Y

	dc := gg.NewContext(x, y)
	dc.AsMask()
	dc.Clear()
	dc.SetRGB(0, 0, 0)
	if err := dc.LoadFontFace(fontPath, cfg.fontSize); err != nil {
		log.Fatalf("Cannot load font from %s: %v", fontPath, err)
	}

	dc.DrawImage(source, 0, 0)
	dc.DrawStringWrapped(cfg.label, cfg.x, cfg.y, cfg.ax, cfg.ay, cfg.width, cfg.spacing, gg.AlignCenter)
	dc.Clip()
	dc.SavePNG(cfg.output)
}

func decodeFileAsPng(f *os.File) image.Image {
	pngImage, err := png.Decode(f)
	if err != nil {
		log.Fatalf("Failed to decode: %s", err)
	}

	return pngImage
}

func getFileName(f string) string {
	name := filepath.Base(f)
	extension := filepath.Ext(f)

	return name[0 : len(name)-len(extension)]
}

// getTrueTypeFonts
func getTrueTypeFonts() []string {
	fonts := findfont.List()

	fontNames := []string{}
	for _, font := range fonts {
		fontNames = append(fontNames, getFileName(font))
	}

	return fontNames
}

func getTrueTypeFont(fontName string) (*truetype.Font, string) {
	fontPath, err := findfont.Find(fontName)
	if err != nil {
		log.Fatalf("Could not find %s font in the System: %v", fontName, err)
	}
	log.Printf("Found '%s' in '%s'\n", fontName, fontPath)

	// load the font with the freetype library
	fontData, err := ioutil.ReadFile(fontPath)
	if err != nil {
		log.Fatalf("Could not read %s font: %v", fontName, err)
	}
	font, err := truetype.Parse(fontData)
	if err != nil {
		log.Fatalf("Could not parse truetype %s font: %v", fontName, err)
	}

	return font, fontPath
}

func openFile(f string) *os.File {
	file, err := os.Open(f)
	if err != nil {
		log.Fatalf("Failed to open: %s", err)
	}

	return file
}

func pathExists(p string) error {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return errors.New("Path does not exist")
	}

	return nil
}

func processImage(cfg cardConfig, imagePath string) {
	templateFile := openFile(cfg.Template)
	templatePng := decodeFileAsPng(templateFile)
	defer templateFile.Close()

	imageFile := openFile(imagePath)
	imagePng := decodeFileAsPng(imageFile)
	defer imageFile.Close()

	offset := image.Pt(22, 22)
	bounds := templatePng.Bounds()

	cardPng := image.NewRGBA(bounds)

	draw.Draw(cardPng, bounds, templatePng, image.ZP, draw.Src)
	draw.Draw(cardPng, imagePng.Bounds().Add(offset), imagePng, image.ZP, draw.Over)

	imageName := getFileName(imagePath)

	outputFilePath := path.Join(cfg.OutputsPath, "card_"+imageName+".png")

	cardFile, err := os.Create(outputFilePath)
	if err != nil {
		log.Fatalf("failed to create: %s", err)
	}

	pngEnconder := png.Encoder{
		CompressionLevel: png.BestCompression,
	}
	pngEnconder.Encode(cardFile, cardPng)
	defer cardFile.Close()

	textFilePath := path.Join(cfg.TextsPath, imageName+".txt")
	texts := readTextFile(textFilePath)

	x := cardPng.Bounds().Max.X
	y := cardPng.Bounds().Max.Y

	defaultWidth := float64(x - 10)

	headerTextCfg := textConfig{
		font:     cfg.HeaderFont,
		label:    texts["header"],
		fontSize: 14,
		x:        float64(x / 2),
		y:        float64(y/2) + 20,
		ax:       0.5,
		ay:       0.5,
		spacing:  1.0,
		output:   outputFilePath,
		width:    defaultWidth,
	}
	titleTextCfg := textConfig{
		font:     cfg.TitleFont,
		label:    texts["title"],
		fontSize: 12,
		x:        float64(x / 2),
		y:        float64(y/2) + 50,
		ax:       0.5,
		ay:       0.5,
		spacing:  1.0,
		output:   outputFilePath,
		width:    defaultWidth,
	}
	bodyTextCfg := textConfig{
		font:     cfg.BodyFont,
		label:    texts["body"],
		fontSize: 10,
		x:        float64(x / 2),
		y:        float64(y/2) + 80,
		ax:       0.5,
		ay:       0.5,
		spacing:  1.0,
		output:   outputFilePath,
		width:    defaultWidth,
	}

	textConfigs := []textConfig{
		headerTextCfg, titleTextCfg, bodyTextCfg,
	}

	for _, cfg := range textConfigs {
		outputFile := openFile(outputFilePath)
		source := decodeFileAsPng(outputFile)
		addText(source, cfg)
	}
}

func processImages(cfg cardConfig) {
	images, _ := ioutil.ReadDir(cfg.ImagesPath)
	for _, img := range images {
		imgPath := path.Join(cfg.ImagesPath, img.Name())

		processImage(cfg, imgPath)
	}
}

func promptPath(label string, defaultValue string) string {
	prompt := promptui.Prompt{
		Label:    "Location of the " + label + ": ",
		Default:  defaultValue,
		Validate: pathExists,
	}

	result, err := prompt.Run()

	if err != nil {
		log.Fatalf("Prompt failed %v\n", err)
	}

	fmt.Printf("You choose %q\n", result)

	return result
}

func promptBasePath() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatalf("Cannot find current user %v", err)
	}

	return promptPath("root directory", usr.HomeDir)
}

func promptImagesPath() string {
	return promptPath("images directory", path.Join(basePath, defaultImagesPath))
}

func promptTemplate(templatesPath string) string {
	items := []string{}

	files, _ := ioutil.ReadDir(templatesPath)
	for _, templateFile := range files {
		if !strings.HasSuffix(templateFile.Name(), ".png") {
			continue
		}

		items = append(items, templateFile.Name())
	}
	if len(items) == 0 {
		log.Fatalf("There are no templates in %s", templatesPath)
	}

	prompt := promptui.Select{
		Label: "Select Template",
		Items: items,
	}

	_, result, err := prompt.Run()

	if err != nil {
		log.Fatalf("Prompt failed %v\n", err)
	}

	fmt.Printf("You choose %q\n", result)

	return result
}

func promptTrueTypeFonts(fontType string) string {
	fonts := getTrueTypeFonts()

	if len(fonts) == 0 {
		log.Fatal("There are no TrueType fonts in the System")
	}

	prompt := promptui.Select{
		Label: "Select " + fontType + " Font",
		Items: fonts,
	}

	_, result, err := prompt.Run()

	if err != nil {
		log.Fatalf("Prompt failed %v\n", err)
	}

	fmt.Printf("You choose %q\n", result)

	return result
}

type cardText map[string]string

func readTextFile(filename string) cardText {
	card := cardText{
		"header": "HEADER",
		"title":  "TITLE",
		"body":   "BODY",
	}
	if len(filename) == 0 {
		log.Fatal("Text file is empty")
	}

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("Cannot open text file: %v", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')

		// check if the line has = sign
		// and process the line. Ignore the rest.
		if equal := strings.Index(line, "="); equal >= 0 {
			if key := strings.TrimSpace(line[:equal]); len(key) > 0 {
				value := ""
				if len(line) > equal {
					value = strings.TrimSpace(line[equal+1:])
				}
				// assign the config map
				card[key] = value
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("Cannot read text file: %v", err)
		}
	}
	return card
}

func validateImages(subDir string, dims imageDimensions) {
	dir := path.Join(basePath, subDir)

	files, _ := ioutil.ReadDir(dir)

	for _, imgFile := range files {
		if !strings.HasSuffix(imgFile.Name(), ".png") {
			log.Fatalf("All images in '%s' must be valid PNG files: %s", subDir, imgFile.Name())
		}

		if reader, err := os.Open(filepath.Join(dir, imgFile.Name())); err == nil {
			defer reader.Close()

			im, _, err := image.DecodeConfig(reader)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", imgFile.Name(), err)
				continue
			}

			if im.Width != dims.x && im.Height != dims.y {
				log.Fatalf("%s does not match dimensions (%d x %d pixels): %v", imgFile.Name(), dims.x, dims.y, err)
			}

		} else {
			log.Fatalf("Impossible to open the file: %v", err)
		}
	}

	log.Printf("All images in '%s' are PNG and satisfy dims (%d x %d px)", subDir, dims.x, dims.y)
}
