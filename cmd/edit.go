package cmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/shoichiimamura/kubase/codec"
	"github.com/spf13/cobra"
	"io/ioutil"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
)

// TODO: 以下のエラーが起こる原因を整理する
// json: error calling MarshalJSON for type *json.RawMessage: invalid character 'p' looking for beginning of value
// http://elliot.land/post/working-with-json-in-go
// 対応する型がなかったり、jsonで"が含まれていなかった場合など
// https://qiita.com/kawaken/items/a389077dae41d37eee1d
// TODO: ポインタ型でないと自己を更新できないので、func (m *RawMessage) UnmarshalJSON(data []byte) error しか実装されていない
// したがって、UnmarshalJSONする場合は、*json.RawMessage型の必要がある
// cf: https://qiita.com/guregu/items/e2e12995c4453fea5603
// TODO: まとめる
// json.RawMessageを使うメリットは、base64 encodeされない
// json.RawMessageを使うデメリットは、UnmarshalJSON時に"が混入する（jsonのvalueは"で囲まれているため）
// https://stackoverflow.com/questions/44125690/how-do-i-convince-unmarshaljson-to-work-with-a-slice-subtype
// Time型のゼロ値は、nilなどではないのでomitemptyの対象にならずゼロ値がmarshalされる
// https://golang.org/pkg/encoding/json/#Marshal
// https://stackoverflow.com/questions/32643815/golang-json-omitempty-with-time-time-field
// https://github.com/kubernetes/kubernetes/issues/67610
// 同じ名前でないと上書きできない(ObjectMetaを上書きたい)
// struct { ObjectMeta } => struct { ObjectMeta md }

type rawValue []byte

// Yamlでも一旦JSONに変換されているので、MarshalJSONインターフェースの実装のみでいい
// https://github.com/kubernetes/apimachinery/blob/2a7c9300402896b3c073f2f47df85527c94f83a0/pkg/runtime/serializer/json/json.go#L223
func (d rawValue) MarshalJSON() ([]byte, error) {
	// jsonをencodeする際に、[]byte(d) だと以下のエラーになる
	// json: error calling MarshalJSON for type *cmd.rawValue: invalid character 'p' looking for beginning of value
	return []byte(`"` + string(d) + `"`), nil
}

func (d *rawValue) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.New("rawValue: UnmarshalJSON on nil pointer")
	}
	// 余計なダブルコーテーションが含まれてしまうので最初と最後を取り除く
	*d = rawValue(data[1 : len(data)-1])
	return nil
}

type objectMeta struct {
	meta.ObjectMeta
	CreationTimestamp *meta.Time `json:"creationTimestamp,omitempty"`
}

type rawSecret struct {
	v1.Secret
	ObjectMeta objectMeta           `json:"metadata,omitempty"`
	Data       map[string]*rawValue `json:"data"`
}

func NewEditCommand() *cobra.Command {
	command := &cobra.Command{
		Use: "edit",
		Run: edit,
	}
	return command
}

func edit(cmd *cobra.Command, args []string) {
	originalFilePath := args[0]

	// create temporary directory
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	// create temporary file
	tempfilePath := path.Join(tmpdir, originalFilePath)
	if _, err := os.Create(tempfilePath); err != nil {
		log.Fatal(err)
	}

	if err := writeConvertedData(originalFilePath, tempfilePath, decodeBase64Data); err != nil {
		log.Fatal(err)
	}

	// let user edit
	if err := runEdit(tempfilePath); err != nil {
		log.Fatal(err)
	}

	// write tempfile data in original file
	// TODO: keyの順序を保つ
	// https://golang.org/pkg/encoding/json/#RawMessage
	if err := writeConvertedData(tempfilePath, originalFilePath, encodeDataByBase64); err != nil {
		log.Fatal(err)
	}
}

func decodeBase64Data(o runtime.Object) error {
	rs, ok := o.(*rawSecret)
	if !ok {
		return fmt.Errorf("失敗")
	}
	for k, v := range rs.Data {
		rv, err := base64.StdEncoding.DecodeString(string(*v))
		if err != nil {
			return err
		}
		rvv := rawValue(rv)
		rs.Data[k] = &rvv
	}
	return nil
}

func encodeDataByBase64(o runtime.Object) error {
	rs, ok := o.(*rawSecret)
	if !ok {
		return fmt.Errorf("失敗")
	}
	for k, v := range rs.Data {
		rv := rawValue(base64.StdEncoding.EncodeToString(*v))
		rs.Data[k] = &rv
	}
	return nil
}

func writeConvertedData(src, dist string, converter func(rs runtime.Object) error) error {
	// read file
	d, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	sc := codec.NewSecretCodec(d)
	b, err := convertSecretFile(sc, converter)
	if err != nil {
		return err
	}
	// write tempfile data in original file
	if err := ioutil.WriteFile(dist, b, os.ModeType); err != nil {
		return err
	}
	return nil
}

func convertSecretFile(sc codec.SecretCodec, converter func(rs runtime.Object) error) ([]byte, error) {
	o, _, err := sc.Decode(&rawSecret{})
	if err != nil {
		return nil, err
	}
	// encode with base64
	if err := converter(o); err != nil {
		return nil, err
	}
	bb, e := sc.Encode(o)
	if e != nil {
		return nil, e
	}
	fmt.Println(string(bb))
	return bb, nil
}

func runEdit(path string) error {
	var cmd *exec.Cmd
	cmd = exec.Command("which", "vim", "nano")
	out, err := cmd.Output()
	if err != nil {
		panic("Could not find any editors")
	}
	fmt.Println(strings.Split(string(out), "\n"))
	cmd = exec.Command(strings.Split(string(out), "\n")[0], path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
