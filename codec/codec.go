package codec

import (
	"bytes"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/recognizer"
)

type SecretCodec struct {
	input   []byte
	factory serializer.CodecFactory
}

func NewSecretCodec(data []byte) SecretCodec {
	cf := serializer.NewCodecFactory(runtime.NewScheme())
	return SecretCodec{factory: cf, input: data}
}

// TODO: 引数の変数名をわかりやすくする
func (c *SecretCodec) Decode(obj runtime.Object) (runtime.Object, *schema.GroupVersionKind, error) {
	return c.factory.UniversalDeserializer().Decode(c.input, nil, obj)
}

// https://github.com/kubernetes/apimachinery/blob/2a7c9300402896b3c073f2f47df85527c94f83a0/pkg/runtime/serializer/codec_factory.go#L47
// https://github.com/kubernetes/apimachinery/blob/2a7c9300402896b3c073f2f47df85527c94f83a0/pkg/runtime/serializer/json/json.go#L38
// https://github.com/kubernetes/apimachinery/blob/2a7c9300402896b3c073f2f47df85527c94f83a0/pkg/runtime/interfaces.go#L97
func (c *SecretCodec) Encode(obj runtime.Object) ([]byte, error) {
	return encode(c.factory.SupportedMediaTypes(), c.input, obj)
}

func encode(info []runtime.SerializerInfo, data []byte, obj runtime.Object) ([]byte, error) {
	var (
		lastErr  error
		encoders []runtime.Encoder
	)
	encoders, lastErr = encodersCorrespondingInput(info, data)
	for _, encoder := range encoders {
		out, err := runtime.Encode(encoder, obj)
		if err != nil {
			lastErr = err
			continue
		}
		return out, nil
	}
	return []byte{}, lastErr
}

type recognizingSerializer interface {
	runtime.Encoder
	recognizer.RecognizingDecoder
}

// Decodeに使われたSerializerのEncoderを利用する
func encodersCorrespondingInput(info []runtime.SerializerInfo, data []byte) ([]runtime.Encoder, error) {
	var (
		lastErr  error
		encoders []runtime.Encoder
		se       runtime.Serializer
	)
	for _, i := range info {
		if !i.EncodesAsText {
			continue
		}
		if i.PrettySerializer != nil {
			se = i.PrettySerializer
		} else {
			se = i.Serializer
		}
		switch e := se.(type) {
		case recognizingSerializer:
			buf := bytes.NewBuffer(data)
			ok, unknown, err := e.RecognizesData(buf)
			if err != nil {
				lastErr = err
				continue
			}
			if unknown {
				encoders = append(encoders, e)
				continue
			}
			if !ok {
				continue
			}
			return []runtime.Encoder{e}, nil
		}
	}
	if len(encoders) == 0 && lastErr == nil {
		lastErr = errors.New("no serialization format matched the provided data")
	}
	return encoders, lastErr
}
