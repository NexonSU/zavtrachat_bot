package main

import (
	"fmt"
	"strconv"
	"strings"

	cmc "github.com/miguelmota/go-coinmarketcap/pro/v1"
	tele "gopkg.in/telebot.v3"
)

var CryptoMap []*cmc.MapListing
var FiatMap []*cmc.FiatMapListing
var CustomMap = []CustomMapStruct{}
var _ = GenerateMaps()

type CustomMapStruct struct {
	symbol string
	name   string
	amount float64
}

func GenerateMaps() error {
	if Config.CurrencyKey == "" {
		return nil
	}
	var err error
	client := cmc.NewClient(&cmc.Config{ProAPIKey: Config.CurrencyKey})
	CryptoMap, err = client.Cryptocurrency.Map(&cmc.MapOptions{ListingStatus: "active,untracked"})
	if err != nil {
		return err
	}
	FiatMap, err = client.Fiat.Map(&cmc.FiatMapOptions{IncludeMetals: true})
	if err != nil {
		return err
	}
	CustomMap = append(CustomMap, CustomMapStruct{symbol: "COC", name: "Cup Of Coffee", amount: 300.0})
	CustomMap = append(CustomMap, CustomMapStruct{symbol: "DSHK", name: "Doshirak", amount: 80.0})
	CustomMap = append(CustomMap, CustomMapStruct{symbol: "DOSH", name: "Doshirak", amount: 80.0})
	CustomMap = append(CustomMap, CustomMapStruct{symbol: "TBW", name: "Boeing Wing", amount: 178000000.0})
	return nil
}

func GetSymbolId(symbol string) (string, error) {
	symbol = strings.ToUpper(symbol)
	if symbol == "BYR" {
		symbol = "BYN"
	}
	if symbol == "TL" {
		symbol = "TRY"
	}
	for _, CustomFiat := range CustomMap {
		if symbol == CustomFiat.symbol {
			symbol = "RUB"
		}
	}
	for _, fiat := range FiatMap {
		if fiat.Symbol == symbol {
			return fmt.Sprintf("%v", int(fiat.ID)), nil
		}
	}
	for _, crypto := range CryptoMap {
		if crypto.Symbol == symbol {
			return fmt.Sprintf("%v", int(crypto.ID)), nil
		}
	}
	return "", fmt.Errorf("не удалось распознать валюту <code>%v</code>", symbol)
}

func GetIdName(ID string) string {
	ID_int, _ := strconv.Atoi(ID)
	for _, fiat := range FiatMap {
		if int(fiat.ID) == ID_int {
			return fiat.Name
		}
	}
	for _, crypto := range CryptoMap {
		if int(crypto.ID) == ID_int {
			return crypto.Name
		}
	}
	return ""
}

// Reply currency "cur"
func Cur(context tele.Context) error {
	if Config.CurrencyKey == "" {
		return ReplyAndRemove("Конвертация валют не настроена", context)
	}
	if len(context.Args()) != 3 {
		return ReplyAndRemove("Пример использования:\n/cur 1 USD RUB", context)
	}
	amount, err := strconv.ParseFloat(context.Args()[0], 64)
	if err != nil {
		return err
	}
	symbol, err := GetSymbolId(context.Args()[1])
	if err != nil {
		return err
	}
	convert, err := GetSymbolId(context.Args()[2])
	if err != nil {
		return err
	}
	for _, CustomFiat := range CustomMap {
		if strings.ToUpper(context.Args()[1]) == CustomFiat.symbol {
			amount = amount * CustomFiat.amount
		}
	}
	if symbol == "9911" || convert == "9911" {
		return fmt.Errorf("невозможно конвертировать тестовую валюту")
	}
	client := cmc.NewClient(&cmc.Config{ProAPIKey: Config.CurrencyKey})
	conversion, err := client.Tools.PriceConversion(&cmc.ConvertOptions{Amount: amount, ID: symbol, ConvertID: convert})
	if err != nil {
		return err
	}
	resultAmount := conversion.Quote[convert].Price
	resultName := GetIdName(convert)
	for _, CustomFiat := range CustomMap {
		if strings.ToUpper(context.Args()[1]) == CustomFiat.symbol {
			conversion.Amount = amount / CustomFiat.amount
			conversion.Name = CustomFiat.name
		}
		if strings.ToUpper(context.Args()[2]) == CustomFiat.symbol {
			resultAmount = resultAmount / CustomFiat.amount
			resultName = CustomFiat.name
		}
	}
	return context.Send(fmt.Sprintf("%.2f %v = %.2f %v", conversion.Amount, conversion.Name, resultAmount, resultName), &tele.SendOptions{ReplyTo: context.Message().ReplyTo, AllowWithoutReply: true})
}
