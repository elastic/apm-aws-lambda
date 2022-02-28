package extension

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"elastic/apm-lambda-extension/model"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
)

type MetadataContainer struct {
	Metadata *model.Metadata `json:"metadata"`
}

func ProcessMetadata(data AgentData, container *MetadataContainer) {
	uncompressedData, err := getUncompressedBytes(data.Data, data.ContentEncoding)
	log.Println(string(uncompressedData))
	if err != nil {
		log.Printf("Error uncompressing agent data for metadata extraction : %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(uncompressedData))
	for {
		err = decoder.Decode(container)
		if container.Metadata != nil {
			log.Printf("Metadata decoded")
			break
		}
		if err != nil {
			if err == io.EOF {
				log.Printf("No metadata in current agent transaction")
			} else {
				log.Printf("Error uncompressing agent data for metadata extraction : %v", err)
			}
		}
	}
}

func getUncompressedBytes(rawBytes []byte, encodingType string) ([]byte, error) {
	switch encodingType {
	case "deflate":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := zlib.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create zlib.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from zlib reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	case "gzip":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("could not create gzip.NewReader: %v", err)
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			return nil, fmt.Errorf("could not read from gzip reader using ioutil.ReadAll: %v", err)
		}
		return bodyBytes, nil
	default:
		return rawBytes, nil
	}
}
