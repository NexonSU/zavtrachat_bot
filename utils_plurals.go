package main

import (
	"log"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Initialing all possible messages with different plurals variants
func init() {
	err := message.Set(language.Russian, "<code>💥 %v %v%v.\nРеспавн через %d мин.</code>",
		plural.Selectf(4, "%d",
			plural.One, "<code>💥 %v %v%v.\nРеспавн через %d минуту.</code>",
			plural.Few, "<code>💥 %v %v%v.\nРеспавн через %d минуты.</code>",
			plural.Many, "<code>💥 %v %v%v.\nРеспавн через %d минут.</code>",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)
	}

	err = message.Set(language.Russian, "💥 %v %vпристрелил %v.\n%v отправился на респавн на %d мин.",
		plural.Selectf(5, "%d",
			plural.One, "💥 %v %vпристрелил %v.\n%v отправился на респавн на %d минуту.",
			plural.Few, "💥 %v %vпристрелил %v.\n%v отправился на респавн на %d минуты.",
			plural.Many, "💥 %v %vпристрелил %v.\n%v отправился на респавн на %d минут.",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)

	}

	err = message.Set(language.Russian, "🤫 %v %vпопросил %v помолчать %d минут.",
		plural.Selectf(4, "%d",
			plural.One, "🤫 %v %vпопросил %v помолчать %d минуту.",
			plural.Few, "🤫 %v %vпопросил %v помолчать %d минуты.",
			plural.Many, "🤫 %v %vпопросил %v помолчать %d минут.",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)

	}

	err = message.Set(language.Russian, "У тебя %d предупреждений.",
		plural.Selectf(1, "%d",
			plural.One, "У тебя %d предупреждение.",
			plural.Few, "У тебя %d предупреждения.",
			plural.Many, "У тебя %d предупреждений.",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)

	}

	err = message.Set(language.Russian, "%v😈 Наводит револьвер на %v и стреляет.\nЯ хз как это объяснить, но %v победитель!\n%v отправился на респавн на %d мин.",
		plural.Selectf(5, "%d",
			plural.One, "%v😈 Наводит револьвер на %v и стреляет.\nЯ хз как это объяснить, но %v победитель!\n%v отправился на респавн на %d минуту.",
			plural.Few, "%v😈 Наводит револьвер на %v и стреляет.\nЯ хз как это объяснить, но %v победитель!\n%v отправился на респавн на %d минуты.",
			plural.Many, "%v😈 Наводит револьвер на %v и стреляет.\nЯ хз как это объяснить, но %v победитель!\n%v отправился на респавн на %d минут.",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)

	}

	err = message.Set(language.Russian, "%v\nПобедитель дуэли: %v.\n%v отправился на респавн на %d мин.",
		plural.Selectf(4, "%d",
			plural.One, "%v\nПобедитель дуэли: %v.\n%v отправился на респавн на %d минуту.",
			plural.Few, "%v\nПобедитель дуэли: %v.\n%v отправился на респавн на %d минуты.",
			plural.Many, "%v\nПобедитель дуэли: %v.\n%v отправился на респавн на %d минут.",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)
	}

	err = message.Set(language.Russian, "%d смертей",
		plural.Selectf(1, "%d",
			plural.One, "%d смерть",
			plural.Few, "%d смерти",
			plural.Many, "%d смертей",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)

	}

	err = message.Set(language.Russian, "%d побед",
		plural.Selectf(1, "%d",
			plural.One, "%d победа",
			plural.Few, "%d победы",
			plural.Many, "%d побед",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)

	}

	err = message.Set(language.Russian, "%v. %v - %d раз\n",
		plural.Selectf(3, "%d",
			plural.One, "%v. %v - %d раз\n",
			plural.Few, "%v. %v - %d раза\n",
			plural.Many, "%v. %v - %d раз\n",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)
	}

	err = message.Set(language.Russian, "В этом году ты был пидором дня — %d раз",
		plural.Selectf(1, "%d",
			plural.One, "В этом году ты был пидором дня — %d раз",
			plural.Few, "В этом году ты был пидором дня — %d раза",
			plural.Many, "В этом году ты был пидором дня — %d раз",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)
	}

	err = message.Set(language.Russian, "За всё время ты был пидором дня — %d раз!",
		plural.Selectf(1, "%d",
			plural.One, "За всё время ты был пидором дня — %d раз!",
			plural.Few, "За всё время ты был пидором дня — %d раза!",
			plural.Many, "За всё время ты был пидором дня — %d раз!",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)
	}

	err = message.Set(language.Russian, "\nВсего участников — %d",
		plural.Selectf(1, "%d",
			plural.One, "\nВсего %d участник",
			plural.Few, "\nВсего %d участника",
			plural.Many, "\nВсего %d участников",
		))
	if err != nil {
		log.Printf("Failed to created plurals template with error: %s\n Failing back to default format", err)
	}
}
