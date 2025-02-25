//go:build !arm && !windows

package builtin

import (
	"context"
	"image"
	"runtime"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	inf "go.viam.com/rdk/ml/inference"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/vision/classification"
)

var LABEL_OUTPUT_MISMATCH = errors.New("Invalid Label File: Number of labels does not match number of model outputs. Labels must be separated by a newline, comma or space.")

// TFLiteClassifierConfig specifies the fields necessary for creating a TFLite classifier.
type TFLiteClassifierConfig struct {
	// this should come from the attributes part of the classifier config
	ModelPath  string  `json:"model_path"`
	NumThreads int     `json:"num_threads"`
	LabelPath  *string `json:"label_path"`
}

// NewTFLiteClassifier creates an RDK classifier given a VisModelConfig. In other words, this
// function returns a function from image-->[]classifier.Classifications. It does this by making calls to
// an inference package and wrapping the result.
func NewTFLiteClassifier(ctx context.Context, conf *vision.VisModelConfig,
	logger golog.Logger,
) (classification.Classifier, *inf.TFLiteStruct, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::NewTFLiteDetector")
	defer span.End()

	// Read those parameters into a TFLiteClassifierConfig
	tfParams, err := resource.TransformAttributeMap[*TFLiteClassifierConfig](conf.Parameters)
	if err != nil {
		return nil, nil, errors.New("error getting parameters from config")
	}
	// Secret but hard limit on num_threads
	if tfParams.NumThreads > runtime.NumCPU()/4 {
		tfParams.NumThreads = runtime.NumCPU() / 4
	}

	model, err := addTFLiteModel(ctx, tfParams.ModelPath, &tfParams.NumThreads)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "something wrong with adding the model")
	}

	if tfParams.LabelPath == nil {
		blank := ""
		tfParams.LabelPath = &blank
	}
	labels, err := loadLabels(*tfParams.LabelPath)
	if err != nil {
		logger.Warn("did not retrieve class labels")
	}

	var inHeight, inWidth uint
	if shape := model.Info.InputShape; getIndex(shape, 3) == 1 {
		inHeight, inWidth = uint(shape[2]), uint(shape[3])
	} else {
		inHeight, inWidth = uint(shape[1]), uint(shape[2])
	}

	// This function that gets returned should be the Classifier
	return func(ctx context.Context, img image.Image) (classification.Classifications, error) {
		// resize the image according to the expected dims
		resizedImg := resize.Resize(inHeight, inWidth, img, resize.Bilinear)
		outTensor, err := tfliteInfer(ctx, model, resizedImg)
		if err != nil {
			return nil, err
		}

		classifications, err := unpackClassificationTensor(ctx, outTensor, model, labels)
		if err != nil {
			return nil, err
		}
		return classifications, nil
	}, model, nil
}

func unpackClassificationTensor(ctx context.Context, tensor []interface{},
	model *inf.TFLiteStruct, labels []string,
) (classification.Classifications, error) {
	_, span := trace.StartSpan(ctx, "service::vision::unpackClassificationTensor")
	defer span.End()

	outType := model.Info.OutputTensorTypes[0]
	var outConf []float64

	switch outType {
	case "UInt8":
		for _, t := range tensor[0].([]uint8) {
			outConf = append(outConf, float64(t)/float64(256))
		}
	case "Float32":
		for _, t := range tensor[0].([]float32) {
			outConf = append(outConf, float64(t))
		}
	default:
		return nil, errors.New("output type not valid. try uint8 or float32")
	}

	out := make(classification.Classifications, 0, len(outConf))
	if len(labels) > 0 {
		if len(labels) != len(outConf) {
			return nil, LABEL_OUTPUT_MISMATCH
		}

		for i, c := range outConf {
			out = append(out, classification.NewClassification(c, labels[i]))
		}
	} else {
		for i, c := range outConf {
			out = append(out, classification.NewClassification(c, strconv.Itoa(i)))
		}
	}
	return out, nil
}
