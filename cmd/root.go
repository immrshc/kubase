package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubernetes/pkg/apis/core"
	"log"
	"os"
	"path/filepath"
)

// 共通フラグ用の変数
// var debug bool
// var name1 string

// 読み込む設定の型
// type Config struct {
// 	ApplicationName string
// 	Debug           bool
// }

// 読み込む設定ファイル名
// var configFile string

// 読み込んだ設定ファイルの構造体
// var config Config

var rootCmd = &cobra.Command{
	Use: "kubase",
	Run: root,
}

// ファイル名
var fileName string

// ファイル読み込みのタイミングでフラグを定義する
func init() {
	// -d filename でdecodeする
	rootCmd.Flags().StringVarP(&fileName, "decode", "d", "", "decode base64 file")
	// 設定ファイル名をフラグで受け取る
	// rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "./config.yaml", "config file name")

	// 設定ファイルの ApplicationName 項目をフラグで上書きする
	// rootCmd.PersistentFlags().String("name", "", "application name")
	// viper.BindPFlag("ApplicationName", rootCmd.PersistentFlags().Lookup("name"))

	// cobra.Command 実行前の初期化処理を定義する。
	// rootCmd.Execute > コマンドライン引数の処理 > cobra.OnInitialize > rootCmd.Run という順に実行されるので、
	// フラグでうけとった設定ファイル名を使って設定ファイルを読み込み、コマンド実行時に設定ファイルの内容を利用することができる。
	cobra.OnInitialize(func() {

		// 設定ファイル名を viper に定義する
		// viper.SetConfigFile(configFile)

		// env
		// viper.AutomaticEnv()

		// 設定ファイルを読み込む
		// if err := viper.ReadInConfig(); err != nil {
		// 	fmt.Println(err)
		// 	os.Exit(1)
		// }
		//
		// // 設定ファイルの内容を構造体にコピーする
		// if err := viper.Unmarshal(&config); err != nil {
		// 	fmt.Println(err)
		// 	os.Exit(1)
		// }
	})

	// コマンド共通のフラグを定義
	// rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "debug enable flag")
	// フラグの値を変数にセットする場合
	// 第1引数: 変数のポインタ
	// 第2引数: フラグ名
	// 第3引数: デフォルト値
	// 第4引数: 説明
	// rootCmd.PersistentFlags().StringVar(&name1, "name1", "default", "your name1")

	// フラグの値をフラグ名で参照する場合
	// 第1引数: フラグ名
	// 第2引数: 短縮フラグ名（末尾が "P" の関数では短縮フラグを指定できる）
	// 第3引数: デフォルト値
	// 第4引数: 説明
	// rootCmd.PersistentFlags().StringP("name2", "n", "default", "your name2")
}

type Secret struct {
	core.Secret
	Data map[string][]byte
}

func root(c *cobra.Command, args []string) {
	fmt.Printf("fileName: %v\n", fileName)
	fmt.Printf("ext: %v\n", filepath.Ext(fileName))
	f, err := os.Open("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	d := yaml.NewYAMLOrJSONDecoder(f, 4096)
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		fmt.Println("raw: ", string(ext.Raw))
	}
}

func Execute() {
	rootCmd.Execute()
}
