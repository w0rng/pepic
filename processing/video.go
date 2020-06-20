package processing

import (
	"errors"
	"fmt"
	"github.com/vas3k/pepic/config"
	"github.com/xfrr/goffmpeg/transcoder"
	"io/ioutil"
	"log"
	"mime"
	"os"
	"path"
	"strings"
)

func isVideo(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}

func transcodeVideo(file *ProcessedFile, maxLength int) error {
	log.Printf("Transcoding video '%s' to %d px", file.Filename, maxLength)
	if file.Data == nil {
		return errors.New("file data is empty, try reading it first")
	}

	// save bytes to disc because ffmpeg works with filenames
	tempOrigFile := path.Join(config.App.Videos.FFmpeg.TempDir, file.Filename)
	dst, err := os.Create(tempOrigFile)
	if err != nil {
		return err
	}
	defer dst.Close()
	defer os.Remove(tempOrigFile)

	_, err = dst.Write(file.Data)
	if err != nil {
		return err
	}

	// create temp file output
	tempTransFile := path.Join(config.App.Videos.FFmpeg.TempDir, fmt.Sprintf("trans_%s", file.Filename))
	dst, err = os.Create(tempTransFile)
	if err != nil {
		return err
	}
	defer dst.Close()
	defer os.Remove(tempTransFile)

	// create and configure video transcoder
	trans, err := initTranscoder(tempOrigFile, tempTransFile)
	if err != nil {
		return err
	}

	// add resize filter
	trans.MediaFile().SetVideoFilter(fmt.Sprintf("scale=trunc(oh*a/2)*2:%d", maxLength))

	// run transcoding and monitor the process
	done := runTranscoder(trans)
	err = <-done
	if err != nil {
		return err
	}

	// load transcoded video back to memory and remove temp files (deferred)
	file.Data, err = ioutil.ReadFile(tempTransFile)
	if err != nil {
		return err
	}

	return nil
}

func convertVideo(file *ProcessedFile, newMimeType string) error {
	log.Printf("Converting video '%s' to %s", file.Filename, newMimeType)
	if file.Data == nil {
		return errors.New("file data is empty, try reading it first")
	}

	if !isVideo(newMimeType) {
		return errors.New(fmt.Sprintf("'%s' is not supported video type", newMimeType))
	}

	// save bytes to disc because ffmpeg works with filenames
	tempOrigFile := path.Join(config.App.Videos.FFmpeg.TempDir, file.Filename)
	dst, err := os.Create(tempOrigFile)
	if err != nil {
		return err
	}
	defer dst.Close()
	defer os.Remove(tempOrigFile)

	_, err = dst.Write(file.Data)
	if err != nil {
		return err
	}

	// create temp file output
	ext, _ := mime.ExtensionsByType(newMimeType)
	newExt := ext[0]
	convFilename := replaceExt(file.Filename, newExt)
	tempTransFile := path.Join(config.App.Videos.FFmpeg.TempDir, fmt.Sprintf("conv_%s", convFilename))
	dst, err = os.Create(tempTransFile)
	if err != nil {
		return err
	}
	defer dst.Close()
	defer os.Remove(tempTransFile)

	// create and configure video transcoder
	trans, err := initTranscoder(tempOrigFile, tempTransFile)
	if err != nil {
		return err
	}

	// run transcoding and monitor the process
	done := runTranscoder(trans)
	err = <-done
	if err != nil {
		return err
	}

	// load transcoded video back to memory and remove temp files (deferred)
	file.Data, err = ioutil.ReadFile(tempTransFile)
	if err != nil {
		return err
	}

	file.Mime = newMimeType
	file.Filename = convFilename
	if file.Path != "" {
		file.Path = replaceExt(file.Path, newExt)
	}

	return nil
}

func initTranscoder(inputPath string, outputPath string) (*transcoder.Transcoder, error) {
	trans := new(transcoder.Transcoder)
	err := trans.Initialize(inputPath, outputPath)
	if err != nil {
		return nil, err
	}

	trans.MediaFile().SetPreset(config.App.Videos.FFmpeg.Preset)
	trans.MediaFile().SetCRF(uint32(config.App.Videos.FFmpeg.CRF))
	trans.MediaFile().SetVideoCodec(config.App.Videos.FFmpeg.VideoCodec)
	trans.MediaFile().SetVideoBitRate(config.App.Videos.FFmpeg.VideoBitrate)
	trans.MediaFile().SetVideoProfile(config.App.Videos.FFmpeg.VideoProfile)
	trans.MediaFile().SetAudioCodec(config.App.Videos.FFmpeg.AudioCodec)
	trans.MediaFile().SetAudioBitRate(config.App.Videos.FFmpeg.AudioBitrate)
	trans.MediaFile().SetBufferSize(config.App.Videos.FFmpeg.BufferSize)
	trans.MediaFile().SetMovFlags(config.App.Videos.FFmpeg.MovFlags)
	trans.MediaFile().SetPixFmt(config.App.Videos.FFmpeg.PixFmt)

	return trans, nil
}

func runTranscoder(trans *transcoder.Transcoder) <-chan error {
	done := trans.Run(true)
	progress := trans.Output()
	for msg := range progress {
		log.Print(msg)
	}
	return done
}
