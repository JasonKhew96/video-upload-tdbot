package main

import (
	"context"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/JasonKhew96/go-tdlib"
	"github.com/disintegration/imaging"
	"gopkg.in/vansante/go-ffprobe.v2"
)

type tdbot struct {
	*tdlib.Client
	chatID      int64
	allProgress map[int32]*progressTracker
	done        chan bool
}

type progressTracker struct {
	lastUpdateTime int64
	videoPath      string
}

func main() {
	tdlib.SetLogVerbosityLevel(1)
	tdlib.SetFilePath("./errors.txt")

	apiID := os.Getenv("API_ID")
	apiHash := os.Getenv("API_HASH")
	botToken := os.Getenv("BOT_TOKEN")

	envChatID := os.Getenv("CHAT_ID")

	if apiID == "" || apiHash == "" || botToken == "" || envChatID == "" {
		log.Fatalln("ENVs is not correct!")
	}
	chatID, err := strconv.ParseInt(envChatID, 10, 64)
	if err != nil {
		log.Fatalln("ENVs is not correct!")
	}

	bot := &tdbot{tdlib.NewClient(tdlib.Config{
		APIID:               apiID,
		APIHash:             apiHash,
		SystemLanguageCode:  "en",
		DeviceModel:         "Server",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "1.0.0",
		UseMessageDatabase:  true,
		UseFileDatabase:     true,
		UseChatInfoDatabase: true,
		UseTestDataCenter:   false,
		DatabaseDirectory:   "./tdlib-db",
		FileDirectory:       "./tdlib-files",
		IgnoreFileNames:     false,
	}),
		chatID,
		make(map[int32]*progressTracker),
		make(chan bool),
	}

	for {
		currentState, _ := bot.Authorize()
		if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitPhoneNumberType {
			_, err := bot.CheckAuthenticationBotToken(botToken)
			if err != nil {
				log.Printf("Error check bot token: %v", err)
				return
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateReadyType {
			log.Println("Authorization Ready! Let's rock")
			break
		}
	}

	_, err = bot.GetChat(bot.chatID)
	if err != nil {
		log.Fatalln(err)
	}

	if err := bot.parseAllVideos(); err != nil {
		log.Fatalln(err)
	}

	go bot.progressHandler()

	time.Sleep(10 * time.Second)
	if len(bot.allProgress) > 0 {
		<-bot.done
	}

	log.Println("Done")
}

func (bot *tdbot) progressHandler() {
	stubFilter := func(msg *tdlib.TdMessage) bool {
		return true
	}
	updateFileReceiver := bot.AddEventReceiver(&tdlib.UpdateFile{}, stubFilter, 8)
	for newMsg := range updateFileReceiver.Chan {
		updateMsg := (newMsg).(*tdlib.UpdateFile)
		if updateMsg.File.Remote != nil {
			if updateMsg.File.Remote.IsUploadingActive {
				uploadProgress, ok := bot.allProgress[updateMsg.File.Id]
				if ok && time.Now().Unix()-uploadProgress.lastUpdateTime > 1 {
					log.Printf("Uploading %s, %d / %d\n", uploadProgress.videoPath, updateMsg.File.Remote.UploadedSize, updateMsg.File.ExpectedSize)
					uploadProgress.lastUpdateTime = time.Now().Unix()
				}
			} else if updateMsg.File.Remote.IsUploadingCompleted {
				log.Printf("Upload Completed, uniqueId \"%s\"\n", updateMsg.File.Remote.UniqueId)
				_, ok := bot.allProgress[updateMsg.File.Id]
				if ok {
					delete(bot.allProgress, updateMsg.File.Id)
				}
				if len(bot.allProgress) <= 0 {
					bot.done <- true
				}
			}
		}
	}
}

func (bot *tdbot) parseAllVideos() error {
	dirname := "./tmp/"
	d, err := os.Open(dirname)
	if err != nil {
		return err
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.Mode().IsRegular() {
			if filepath.Ext(file.Name()) == ".mkv" || filepath.Ext(file.Name()) == ".mp4" {
				videoPath := filepath.Join(dirname, file.Name())
				coverPath, err := bot.getCoverFile(getBaseFilename(file.Name()))
				if err != nil {
					return err
				}
				if coverPath == "" {
					log.Println(file.Name() + " does not have cover")
					continue
				}
				if err = bot.sendVideoAlbum(videoPath, coverPath); err != nil {
					log.Println(err)
				}
			}
		}
	}

	return nil
}

func getBaseFilename(filename string) string {
	if filepath.Ext(filename) != "" {
		return filename[:len(filename)-len(filepath.Ext(filename))]
	}
	return filename
}

func (bot *tdbot) getCoverFile(filename string) (string, error) {
	dirname := "./tmp/"

	d, err := os.Open(dirname)
	if err != nil {
		return "", err
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.Mode().IsRegular() {
			if filepath.Ext(file.Name()) == ".png" || filepath.Ext(file.Name()) == ".jpg" || filepath.Ext(file.Name()) == ".jpeg" {
				if strings.Contains(file.Name(), filename) {
					return dirname + file.Name(), nil
				}
			}
		}
	}

	return "", nil
}

func (bot *tdbot) sendVideoAlbum(videoPath, coverPath string) error {
	newCoverPath, dx, dy, err := processCover(coverPath)
	if err != nil {
		return err
	}

	duration, vwidth, vheight, err := getVideoMeta(videoPath)
	if err != nil {
		return err
	}

	inputFileVideo := tdlib.NewInputFileLocal(videoPath)
	inputFileCover := tdlib.NewInputFileLocal(newCoverPath)

	inputThumbnail := tdlib.NewInputThumbnail(inputFileCover, int32(dx), int32(dy))

	inputMsgVideo := tdlib.NewInputMessageVideo(inputFileVideo, inputThumbnail, nil, int32(duration), int32(vwidth), int32(vheight), true, nil, 0)

	msgVideo, err := bot.SendMessage(bot.chatID, 0, 0, nil, nil, inputMsgVideo)
	if err != nil {
		return err
	}

	bot.allProgress[msgVideo.Content.(*tdlib.MessageVideo).Video.Video.Id] = &progressTracker{
		lastUpdateTime: time.Now().Unix() - 5,
		videoPath:      videoPath,
	}

	return nil
}

func processCover(imagePath string) (string, int, int, error) {
	// reader, err := os.Open(imagePath)
	// if err != nil {
	// 	return "", 0, 0, err
	// }
	// defer reader.Close()
	// im, _, err := image.Decode(reader)
	// if err != nil {
	// 	return "", 0, 0, err
	// }

	// scaled := resize.Resize(320, 0, im, resize.Lanczos2)

	// dstFile, err := os.Create(getBaseFilename(imagePath) + ".resize.png")
	// if err != nil {
	// 	return "", 0, 0, err
	// }
	// defer dstFile.Close()
	// err = png.Encode(dstFile, scaled)
	// if err != nil {
	// 	return "", 0, 0, err
	// }

	src, err := imaging.Open(imagePath)
	if err != nil {
		return "", 0, 0, err
	}
	if src.Bounds().Dx() > src.Bounds().Dy() {
		src = imaging.Resize(src, 320, 0, imaging.Lanczos)
	} else {
		src = imaging.Resize(src, 0, 320, imaging.Lanczos)
	}

	newPath := getBaseFilename(imagePath) + ".resize.png"

	err = imaging.Save(src, newPath)
	if err != nil {
		return "", 0, 0, err
	}

	return newPath, src.Bounds().Dx(), src.Bounds().Dy(), nil
}

func getVideoMeta(videoPath string) (float64, int, int, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	data, err := ffprobe.ProbeURL(ctx, videoPath)
	if err != nil {
		return 0, 0, 0, err
	}
	return data.Format.DurationSeconds, data.FirstVideoStream().Width, data.FirstVideoStream().Height, nil
}
