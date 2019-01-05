package cmd

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"github.com/google/shlex"
	"github.com/shoichiimamura/kubase/codec"
	"github.com/shoichiimamura/kubase/errors"
	"github.com/shoichiimamura/kubase/util"
	"github.com/spf13/cobra"
	"io"
	"io/ioutil"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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

var editor string

func NewEditCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "edit file",
		Short: "Edit decoded secret file.",
		Long: util.LongDesc(`
			Edit decoded kubernetes secret manifest, which should be encoded
			by base64.
			
			Value data field of this manifest has should be encode by base64.
			You can edit decoded secret manifest file, using editor. After the
			editor closed, data value is encoded. if you want to select editor
			to edit secret manifest file, you can select it by passing value of
			--editor flag.
		`),
		Example: util.Examples(`
				# Edit yaml or json secret file
				kubase edit secret.yaml

				# Edit secret file by selected editor
				kubase edit secret.json --editor vim
		`),
		Args: func(cmd *cobra.Command, args []string) error {
			return validateArgs(cmd, args)
		},
		Run: func(cmd *cobra.Command, args []string) {
			errors.CheckError(edit(cmd, args))
		},
	}
	command.Flags().StringVarP(&editor, "editor", "e", "", "editor to write decoded file")
	return command
}

func validateArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.ArgumentError.Newf("number of args is over acceptance: %v", args)
	}
	filePath := getFilePathFromArgs(args)
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	if m := info.Mode(); m.IsDir() {
		return errors.FileOperationError.Newf("could not read directory at %s", filePath)
	}
	return nil
}

func getFilePathFromArgs(args []string) string {
	return args[0]
}

func edit(cmd *cobra.Command, args []string) error {
	originalFilePath := getFilePathFromArgs(args)

	// create temporary directory
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		return errors.Newf("failure to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// create temporary file
	tempfilePath := path.Join(tmpdir, filepath.Base(originalFilePath))
	if _, err := os.Create(tempfilePath); err != nil {
		return errors.Wrap(err, "failure to create temporary file")
	}

	// write decoded original file data in temporary file
	if err := writeConvertedData(originalFilePath, tempfilePath, decodeBase64Data); err != nil {
		return err
	}

	return runEditorUntilOk(tempfilePath, originalFilePath)
}

func decodeBase64Data(o runtime.Object) error {
	rs, ok := o.(*rawSecret)
	if !ok {
		return errors.New("failure to typecast for base64 decode")
	}
	for k, v := range rs.Data {
		decodedStr, err := base64.StdEncoding.DecodeString(string(*v))
		if err != nil {
			return errors.Newf("failure to decode %s: %v", string(*v), err)
		}
		rv := rawValue(decodedStr)
		rs.Data[k] = &rv
	}
	return nil
}

func encodeDataByBase64(o runtime.Object) error {
	rs, ok := o.(*rawSecret)
	if !ok {
		return errors.New("failure to typecast for base64 encode")
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
	if err := ioutil.WriteFile(dist, b, 0666); err != nil {
		return errors.Wrapf(err, "failure to write into %s: %v", dist)
	}
	return nil
}

func convertSecretFile(sc codec.SecretCodec, converter func(rs runtime.Object) error) ([]byte, error) {
	o, gvk, err := sc.Decode(&rawSecret{})
	if err != nil {
		return nil, errors.UnmarshalInvalidFormatError.Wrap(err, "could not unmarshal file data to secret")
	}
	if gvk.Kind != "Secret" {
		return nil, errors.Newf("invalid resource kind: %s", gvk.Kind)
	}
	if err := converter(o); err != nil {
		return nil, err
	}
	return sc.Encode(o)
}

type editCommandOption struct {
	dist   string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func runEditorUntilOk(src, dist string) error {
	editOption := editCommandOption{
		dist:   src,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	for {
		// let user edit
		if err := runEdit(editOption); err != nil {
			return err
		}

		// write encoded temporary file data in original file
		if err := writeConvertedData(src, dist, encodeDataByBase64); err != nil {
			if errors.GetType(err) == errors.UnmarshalInvalidFormatError {
				fmt.Fprint(
					editOption.stderr,
					"error: ",
					"Could not load file, probably due to invalid syntax.\n",
					"Press a key to return to the editor, or Ctrl+C to exit.\n",
				)
				bufio.NewReader(editOption.stdin).ReadByte()
				continue
			}
			return err
		}
		break
	}
	return nil
}

func runEdit(option editCommandOption) error {
	var cmd *exec.Cmd
	if editor != "" {
		parts, err := shlex.Split(editor)
		if err != nil {
			return errors.Newf("invalid $EDITOR: %s", editor)
		}
		parts = append(parts, option.dist)
		cmd = exec.Command(parts[0], parts[1:]...)
	} else {
		cmd = exec.Command("which", "vim", "nano")
		out, err := cmd.Output()
		editors := strings.Split(string(out), "\n")
		if err != nil || len(editors) == 0 {
			return errors.New("failure to find any editors")
		}
		cmd = exec.Command(editors[0], option.dist)
	}
	cmd.Stdin = option.stdin
	cmd.Stdout = option.stdout
	cmd.Stderr = option.stderr
	return cmd.Run()
}
