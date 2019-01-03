package cmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/shoichiimamura/kubase/codec"
	"github.com/shoichiimamura/kubase/util"
	"github.com/spf13/cobra"
	"io/ioutil"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"os/exec"
	"path"
	"strings"
)

type rawValue []byte

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

// TODO: explain usage
func NewEditCommand() *cobra.Command {
	command := &cobra.Command{
		Use: "edit",
		Run: func(cmd *cobra.Command, args []string) {
			util.ErrorCheck(edit(cmd, args))
		},
	}
	return command
}

func edit(cmd *cobra.Command, args []string) error {
	// TODO: validate
	originalFilePath := args[0]

	// create temporary directory
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failure to create temporary direcotry: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// create temporary file
	tempfilePath := path.Join(tmpdir, originalFilePath)
	if _, err := os.Create(tempfilePath); err != nil {
		return fmt.Errorf("failure to create temporary file: %v", err)
	}

	// write decoded original file data in temporary file
	if err := writeConvertedData(originalFilePath, tempfilePath, decodeBase64Data); err != nil {
		return err
	}

	// let user edit
	// TODO: Check difference by file hash
	if err := runEdit(tempfilePath); err != nil {
		return err
	}

	// write encoded temporary file data in original file
	if err := writeConvertedData(tempfilePath, originalFilePath, encodeDataByBase64); err != nil {
		return err
	}
	return nil
}

func decodeBase64Data(o runtime.Object) error {
	rs, ok := o.(*rawSecret)
	if !ok {
		return fmt.Errorf("failure to typecast for base64 decode")
	}
	for k, v := range rs.Data {
		decodedStr, err := base64.StdEncoding.DecodeString(string(*v))
		if err != nil {
			return fmt.Errorf("failure to decode %s: %v", string(*v), err)
		}
		rv := rawValue(decodedStr)
		rs.Data[k] = &rv
	}
	return nil
}

func encodeDataByBase64(o runtime.Object) error {
	rs, ok := o.(*rawSecret)
	if !ok {
		return fmt.Errorf("failure to typecast for base64 encode")
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
	// encode or decode secret data
	sc := codec.NewSecretCodec(d)
	b, err := convertSecretFile(sc, converter)
	if err != nil {
		return err
	}
	// write tempfile data in original file
	if err := ioutil.WriteFile(dist, b, os.ModeType); err != nil {
		return fmt.Errorf("failure to write into %s: %v", dist, err)
	}
	return nil
}

func convertSecretFile(sc codec.SecretCodec, converter func(rs runtime.Object) error) ([]byte, error) {
	// TODO: handle syntax error which only written file raised
	o, _, err := sc.Decode(&rawSecret{})
	if err != nil {
		return nil, err
	}
	// TODO: validate kind
	if err := converter(o); err != nil {
		return nil, err
	}
	return sc.Encode(o)
}

// TODO: enable to respond to option
func runEdit(path string) error {
	var cmd *exec.Cmd
	cmd = exec.Command("which", "vim", "nano")
	out, err := cmd.Output()
	editors := strings.Split(string(out), "\n")
	if err != nil || len(editors) == 0 {
		return fmt.Errorf("failure to find any editors")
	}
	cmd = exec.Command(editors[0], path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
