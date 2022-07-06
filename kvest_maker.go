package main

import (
	"flag"
	tb "gopkg.in/tucnak/telebot.v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

//Тип конфига
type Config struct {
	Locations      map[string]Location `yaml:"Locations"`
	Final_location Location            `yaml:"Final_location"`
	Teams          []struct {
		Chat_id int64  `yaml:"Chat_id"`
		Name    string `yaml:"Name"`
		URL     string `yaml:"URL"`
	} `yaml:"Teams"`
	Path_to_files      string  `yaml:"Path_to_files"` //где хранить файлы с пользователями
	Telegram_token     string  `yaml:"Telegram_token"`
	Welcome            string  `yaml:"Welcome"`                      //текст приветсвия
	Welcome_with_photo bool    `yaml:"Welcome_with_photo,omitempty"` //фото с приветсвенным словом
	Welcome_with_video bool    `yaml:"Welcome_with_video,omitempty"` //видео с приветсвенным словом
	Media_from_disk    bool    `yaml:"Media_from_disk,omitempty"`    // фото/видео брать с диска
	Media_from_url     bool    `yaml:"Media_from_url,omitempty"`     // фото/видео брать из ссылки
	Media              string  `yaml:"Media,omitempty"`              // ссылка или путь до файла
	Users_limit        int     `yaml:"Users_limit"`                  // сколько максимум пользователей можемт участвовать в квесте
	Will_play_question string  `yaml:"Will_play_question"`           //текст вопроса для участия в квесте
	Rules              string  `yaml:"Rules"`                        //правила для отправки в группу
	Help_cost          int     `yaml:"Help_cost"`                    //сколько добавлять минут за использование подсказки
	Admins_ids         []int64 `yaml:"Admins_ids"`                   //telegram id ведущего/админа который будет запускать раунды
}

//Тип локации
type Location struct {
	//omitempty - означет, что может отсутсвовать
	With_photo      bool   `yaml:"With_photo,omitempty"`      //вопрос с фото
	With_video      bool   `yaml:"With_video,omitempty"`      //вопрос с видео
	With_audio      bool   `yaml:"With_audio,omitempty"`      //вопрос с аудио
	Media_from_disk bool   `yaml:"Media_from_disk,omitempty"` // фото/видео/аудио брать с диска
	Media_from_url  bool   `yaml:"Media_from_url,omitempty"`  // фото/видео/аудио брать из ссылки
	Is_it_clear     bool   `yaml:"Is_it_clear"`               //чистая ли комната (нужна для того чтобы группы не пересекались и в комнате был порядок)
	Media           string `yaml:"Media,omitempty"`           // ссылка или путь до файла
	Queston         string `yaml:"Queston"`                   //вопрос
	Answer          string `yaml:"Answer"`                    //ответ
	Help            string `yaml:"Help"`                      //подсказка
	Name            string `yaml:"Name"`                      //название локации
}

//Тип группы
type Team struct {
	Chat_id             int64     `yaml:"Chat_id"`
	Time_added          int       `yaml:"Time_added"`          //добавленное время за использование подсказок
	Time_finish         time.Time `yaml:"Time_finish"`         //время финиша
	Users               int       `yaml:"Users"`               //количество пользователей в группе
	Finished_locastions []string  `yaml:"Finished_locastions"` //пройденные локации
	Сurrent_locastion   string    `yaml:"Сurrent_locastion"`   //текущая локация
	URL                 string    `yaml:"URL"`                 //ссылка на группу
	Name                string    `yaml:"Name"`                //имя команды
	Finished            bool      `yaml:"Finished"`            //признак финиша
}

//Тип пользователя
type User struct {
	ID        int64  `yaml:"ID"`
	Nick      string `yaml:"Nick"`
	FirstName string `yaml:"FirstName"`
	LastName  string `yaml:"LastName"`
	Number    int    `yaml:"Number"`
	Team_name string `yaml:"Team_name"`
}

//Тип для записи времени начала
type Time struct {
	Begin time.Time `yaml:"Begin"`
}

var (
	currentTime   string        //текущая дата, обновляется при запуске /start
	config        Config        //конфиг определяется во время запуска
	b             *tb.Bot       //бот чтобы не передать во всех используемых функциях
	config_path   string        //путь до конфига
	chan_users    chan User     //канал для передачи пользователей на запись
	chan_team     chan Team     //канал для передачи групп на запись
	chan_location chan Location //канал для передачи локаций на запись
)

//Читает флаг или задаем значение по умолчанию
func init() {
	flag.StringVar(&config_path, "config", "./config.yaml", "path to config file")
}

func main() {
	//читает конфиг файл и парсит его
	yamlFile, err := ioutil.ReadFile(config_path)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		log.Printf("Unmarshal: %v", err)
	}
	currentTime = time.Now().Format("01-02-2006")
	//настраиваем бота
	b, err = tb.NewBot(tb.Settings{
		Token:  config.Telegram_token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Println("Error connect to bot: ", err)
		return
	}
	//задаем каналы
	chan_users = make(chan User, 10)
	chan_team = make(chan Team, len(config.Teams)*2)
	chan_location = make(chan Location, len(config.Locations))
	//создает кнопки для админа/ведущего
	buttons_with_rounds_for_admin := [][]tb.InlineButton{}
	result_button := tb.InlineButton{
		Unique: "R",
		Text:   "Результаты",
	}
	start_button := tb.InlineButton{
		Unique: "S",
		Text:   "Начать",
	}
	buttons_with_rounds_for_admin = append(buttons_with_rounds_for_admin, []tb.InlineButton{start_button})
	i := 0
	for idx, _ := range config.Locations {
		location_button := tb.InlineButton{
			Unique: strconv.Itoa(i) + "_room",
			Text:   idx + " свободна",
		}
		i += 1
		buttons_with_rounds_for_admin = append(buttons_with_rounds_for_admin, []tb.InlineButton{location_button})
	}
	buttons_with_rounds_for_admin = append(buttons_with_rounds_for_admin, []tb.InlineButton{tb.InlineButton{Text: "---Финиш---"}})
	var team_button tb.InlineButton
	for idx, team := range config.Teams {
		team_button = tb.InlineButton{
			Unique: strconv.Itoa(idx),
			Text:   "Финиш для команды " + team.Name,
		}
		buttons_with_rounds_for_admin = append(buttons_with_rounds_for_admin, []tb.InlineButton{team_button})
	}
	buttons_with_rounds_for_admin = append(buttons_with_rounds_for_admin, []tb.InlineButton{result_button})
	//buttons_with_rounds_for_admin = append(buttons_with_rounds_for_admin, []tb.InlineButton{tb.InlineButton{Text: "Статус", Unique: "Status"}})
	//запускаю обработчик команд, кнопок и входящих сообщений
	hadle_start(buttons_with_rounds_for_admin)
	hadle_messeges()
	for _, button := range buttons_with_rounds_for_admin {
		if strings.Contains(button[0].Text, "Финиш") {
			hadle_team_button(button[0])
		}
		if strings.Contains(button[0].Text, "свободна") {
			hadle_rooms(button[0])
		}
		if button[0].Unique == "R" {
			hadle_resutl(button[0])
		}
		if button[0].Unique == "S" {
			hadle_play(button[0])
		}
		//		if button[0].Unique == "Status" {
		//hadle_status(button[0])
		//		}
	}
	//запускаю отдельным потоком функции записи
	go write_user()
	go write_team()
	go write_location()
	b.Start()
}

//Обработчик команды старт
func hadle_start(buttons_with_rounds_for_admin [][]tb.InlineButton) {
	b.Handle("/start", func(m *tb.Message) {
		//обновляем текущую дату и создаем папку
		currentTime = time.Now().Format("01-02-2006")
		if _, err := os.Stat(config.Path_to_files + currentTime); os.IsNotExist(err) {
			os.Mkdir(config.Path_to_files+currentTime, 0755)
		}
		//создаем папки с группами
		for _, team := range config.Teams {
			if _, err := os.Stat(config.Path_to_files + currentTime + "/teams"); os.IsNotExist(err) {
				os.Mkdir(config.Path_to_files+currentTime+"/teams", 0755)
			}
			if _, err := os.Stat(config.Path_to_files + currentTime + "/teams/" + team.Name + ".yaml"); os.IsNotExist(err) {
				f, err := os.Create(config.Path_to_files + currentTime + "/teams/" + team.Name + ".yaml")
				f.Close()
				if err != nil {
					log.Println("Can't create team file: ", err)
					send_to_admin("Ошибка создания файла группы" + team.Name + ":\n" + err.Error())
				}
				se := new(tb.Chat)
				se.ID = team.Chat_id
				//отправляем правила в группу
				m, err := b.Send(se, config.Rules)
				if err != nil {
					log.Println("Send to", err)
					send_to_admin("Ошибка отправки правил в чат " + team.Name + ":\n" + err.Error())
				}
				//закрепляем правила
				err = b.Pin(m)
				if err != nil {
					log.Println("Error pin message\n", err)
					send_to_admin("Ошибка закрепления сообщения в чате" + team.Name + ":\n" + err.Error())
				}
			} else {
				continue
			}
			//наполняем и записываем в файл группу
			var t Team
			t.Chat_id = team.Chat_id
			t.URL = team.URL
			t.Name = team.Name
			t.Users = 0
			chan_team <- t
		}
		//создаем папки и файлы с локациями
		for idx, location := range config.Locations {
			if _, err := os.Stat(config.Path_to_files + currentTime + "/locations/"); os.IsNotExist(err) {
				os.Mkdir(config.Path_to_files+currentTime+"/locations/", 0755)
			}
			if _, err := os.Stat(config.Path_to_files + currentTime + "/locations/" + idx + ".yaml"); os.IsNotExist(err) {
				f, err := os.Create(config.Path_to_files + currentTime + "/locations/" + idx + ".yaml")
				f.Close()
				if err != nil {
					log.Println("Can't create location file: ", err)
					send_to_admin("Ошибка создания файла локации:\n" + err.Error())
				}
			} else {
				continue
			}
			chan_location <- location
		}
		//проверяет кто отправил команду
		//если отправил админ то отправятся админская клавиатура
		for _, admin_id := range config.Admins_ids {
			if m.Chat.ID == admin_id {
				m, err := b.Send(m.Chat, "Меню", &tb.ReplyMarkup{
					InlineKeyboard: buttons_with_rounds_for_admin,
				})
				if err != nil {
					log.Println("Error while send admin keyboard\n", err)
					send_to_admin("Ошибка отправки клавиатуры админа:\n" + err.Error())
				}
				err = b.Pin(m)
				if err != nil {
					log.Println("Error pin message\n", err)
					send_to_admin("Ошибка закрепления сообщения :\n" + err.Error())
				}
				return
			}
		}
		//подготовка к отправке приветсвенного сообщения
		//с фото
		if config.Welcome_with_photo {
			var a tb.Photo
			//от куда брать фото
			if config.Media_from_url {
				a = tb.Photo{File: tb.FromURL(config.Media), Caption: config.Welcome}
			}
			if config.Media_from_disk {
				a = tb.Photo{File: tb.FromDisk(config.Media), Caption: config.Welcome}
			}
			//отправляет приветсвенное слово с фото
			_, err := b.Send(m.Chat, &a)
			if err != nil {
				log.Println("Send Welcome with photo: ", err)
				send_to_admin("Ошибка отправки приветсвенного сообщения с фото:\n" + err.Error())
			}
			//с видео
		} else if config.Welcome_with_video {
			var a tb.Video
			//от куда брать видео
			if config.Media_from_url {
				a = tb.Video{File: tb.FromURL(config.Media), Caption: config.Welcome}
			}
			if config.Media_from_disk {
				a = tb.Video{File: tb.FromDisk(config.Media), Caption: config.Welcome}
			}
			//отправляет приветсвенное слово с видео
			_, err := b.Send(m.Chat, &a)
			if err != nil {
				log.Println("Send Welcome with Video: ", err)
				send_to_admin("Ошибка отправки приветсвенного сообщения с видео:\n" + err.Error())
			}
		} else {
			//отправляет приветсвенное слово
			_, err := b.Send(m.Chat, config.Welcome)
			if err != nil {
				log.Println("Send Welcome: ", err)
				send_to_admin("Ошибка отправки приветсвенного сообщения:\n" + err.Error())
			}
		}
		//создает пользователя если его не существует
		adduser(config.Path_to_files+currentTime, m.Sender.ID, m.Sender.Username, m.Sender.FirstName, m.Sender.LastName)
		//создаем кнопки для участия в квесте
		will_play_button_for_send := [][]tb.InlineButton{}
		will_play_button := tb.InlineButton{
			Unique: "Y",
			Text:   "ДА!",
		}
		hadle_yes(will_play_button)
		will_play_button_for_send = append(will_play_button_for_send, []tb.InlineButton{will_play_button})
		will_play_button = tb.InlineButton{
			Unique: "N",
			Text:   "Нет(",
		}
		hadle_no(will_play_button)
		will_play_button_for_send = append(will_play_button_for_send, []tb.InlineButton{will_play_button})
		_, err := b.Send(m.Chat, config.Will_play_question, &tb.ReplyMarkup{
			InlineKeyboard: will_play_button_for_send,
		})
		if err != nil {
			log.Println("Send game keyboard: ", err)
			send_to_admin("Ошибка отправки игрового меню:\n" + err.Error())
		}

	})
}

//Обработчик кнопки да с предложением квеста
func hadle_yes(yes tb.InlineButton) {
	b.Handle(&yes, func(c *tb.Callback) {
		//получаем пользователя
		user := get_user(strconv.Itoa(int(c.Sender.ID)))
		if user.Team_name == "" {
			var teams []Team
			users := 0
			//получаем все группы
			for _, team := range config.Teams {
				t := get_team(team.Name)
				teams = append(teams, t)
				users += t.Users
			}
			//Проверяем количество пользователей в группах
			if users < config.Users_limit {
				//сортируем группы по количеству пользователей
				sort.Slice(teams, func(i, j int) bool {
					return teams[i].Users < teams[j].Users
				})
				team_button_for_send := [][]tb.InlineButton{}
				team_button := tb.InlineButton{
					Text: "Ссылка на твою группу",
					URL:  teams[0].URL,
				}
				teams[0].Users += 1
				user.Team_name = teams[0].Name
				//записываем пользователя и группу
				chan_users <- user
				chan_team <- teams[0]
				team_button_for_send = append(team_button_for_send, []tb.InlineButton{team_button})
				_, err := b.Send(c.Sender, "👇🏻Переходите по ссылке, чтобы присоединиться к своей команде!", &tb.ReplyMarkup{
					InlineKeyboard: team_button_for_send,
				})
				if err != nil {
					log.Println("Error send team button:\n", err)
					send_to_admin("Ошибка отправки кнопки команды:\n" + err.Error())
				}
			} else {
				//Если желающих поиграть в квест больше чем мест из конфига то отправляем сообщение
				_, err := b.Send(c.Sender, "Прошу прощение, у нас SOLD OUT")
				if err != nil {
					log.Println("Error send team button:\n", err)
					send_to_admin("Ошибка отправки кнопки команды:\n" + err.Error())
				}
			}
		} else {
			//Если у пользователя уже есть группа
			team := get_team(user.Team_name)
			team_button_for_send := [][]tb.InlineButton{}
			team_button := tb.InlineButton{
				Text: "Ссылка на твою группу",
				URL:  team.URL,
			}
			team_button_for_send = append(team_button_for_send, []tb.InlineButton{team_button})
			_, err := b.Send(c.Sender, "👇🏻Переходите по ссылке, чтобы присоединиться к своей команде!", &tb.ReplyMarkup{
				InlineKeyboard: team_button_for_send,
			})
			if err != nil {
				log.Println("Error send team button:\n", err)
				send_to_admin("Ошибка отправки кнопки команды:\n" + err.Error())
			}
		}
		err := b.Respond(c, &tb.CallbackResponse{})
		if err != nil {
			log.Println("Respons: ", err)
			send_to_admin("Ошибка ответа по кнопке да:\n" + err.Error())
		}
	})
}

//Обработчик кнопки нет с предложением квеста
func hadle_no(no tb.InlineButton) {
	b.Handle(&no, func(c *tb.Callback) {
		_, err := b.Send(c.Sender, "🥲Очень жаль, если передумаете, нажмите «ДА»\n🕺🏻В любом случае, наш ведущий Данил не даст вам скучать!")
		err = b.Respond(c, &tb.CallbackResponse{})
		if err != nil {
			log.Println("Respons: ", err)
			send_to_admin("Ошибка ответа по кнопке нет:\n" + err.Error())
		}
	})
}

//Обработчик кнопки результатов в админском чате
func hadle_resutl(r tb.InlineButton) {
	b.Handle(&r, func(c *tb.Callback) {
		if _, err := os.Stat(config.Path_to_files + currentTime+ "/game_begin_time.yaml"); os.IsNotExist(err) {
			err = b.Respond(c, &tb.CallbackResponse{Text:"Игра еще не началась, подожди"})
			if err != nil {
				log.Println("Respons: ", err)
				send_to_admin("Ошибка ответа по кнопке результатов:\n" + err.Error())
			}
			return
		}
		//читаем время начала
		yamlFile, err := ioutil.ReadFile(config.Path_to_files + currentTime + "/game_begin_time.yaml")
		if err != nil {
			log.Printf("yamlFile.Get err   #%v ", err)
			send_to_admin("Ошибка чтения файла начала отсчета:\n" + err.Error())
		}
		//преобразуем
		var t Time
		err = yaml.Unmarshal(yamlFile, &t)
		if err != nil {
			log.Printf("Unmarshal: %v", err)
			send_to_admin("Ошибка парсинга начала отсчета:\n" + err.Error())
		}
		//получаем массив всех групп
		var teams []Team
		for _, team := range config.Teams {
			teams = append(teams, get_team(team.Name))
		}
		//сортуруем по времени финиша
		sort.Slice(teams, func(i, j int) bool {
			return teams[i].Time_finish.Sub(t.Begin) < teams[j].Time_finish.Sub(t.Begin)
		})
		//готовим сообщение
		var message string
		for _, team := range teams {
			diff := team.Time_finish.Sub(t.Begin)
			out := time.Time{}.Add(diff)
			diff1 := team.Time_finish.Add(time.Minute * time.Duration(team.Time_added)).Sub(t.Begin)
			out1 := time.Time{}.Add(diff1)
			message += "Команда " + team.Name + " прошла за " + out.Format("15:04:05") + " и имеет добавленное время " + strconv.Itoa(team.Time_added) + " минут\nИтого: " + out1.Format("15:04:05") + "\n\n\n"
		}
		_, err = b.Send(c.Message.Chat, message)
		err = b.Respond(c, &tb.CallbackResponse{})
		if err != nil {
			log.Println("Respons: ", err)
			send_to_admin("Ошибка ответа по кнопке результатов:\n" + err.Error())
		}
	})
}

//Обработчик кнопки начать в админском чате
func hadle_play(p tb.InlineButton) {
	b.Handle(&p, func(c *tb.Callback) {
		//получаем список всех групп
		var teams []Team
		for _, team := range config.Teams {
			teams = append(teams, get_team(team.Name))
		}
		for _, team := range teams {
			//ищем свободную локацию для группы
			location := get_free_location_for_team(team)
			//если пришла пустая локация то ждем 10 секунд и повторяем попытку
			for location.Name == "" {
				time.Sleep(10 * time.Second)
				location = get_free_location_for_team(team)
			}
			//меняем статус локации и группы
			location.Is_it_clear = false
			team.Сurrent_locastion = location.Name
			//записываем группу и локацию
			chan_team <- team
			chan_location <- location
			send_to_team(location, team)
		}
		err := b.Respond(c, &tb.CallbackResponse{Text: "Задания отправил!"})
		if err != nil {
			log.Println("Respons: ", err)
			send_to_admin("Ошибка ответа по кнопке начать в админском чате:\n" + err.Error())
		}
		write_begin_time()
		status_updater()
	})
}

//Обновляет сообщение со статусом локаций и групп
func status_updater() {
	//получаем статус
	text := get_status()
	//готовим массив сообщений
	var messages []*tb.Message
	for _, admin_id := range config.Admins_ids {
		//готовим чат
		se := new(tb.Chat)
		se.ID = admin_id
		//отправляем текст
		m, err := b.Send(se, text)
		if err != nil {
			log.Println("Error send error to admin: ", err)
			send_to_admin("Ошибка отправки ошибки:\n" + err.Error())
		}
		//сохранаяем сообщение в массив
		messages = append(messages, m)
	}
	for _, message := range messages {
		go func(message *tb.Message) {
			for {
				//ждем минуту
				time.Sleep(time.Minute * 1)
				//получаем новый статус
				new_text := get_status()
				if new_text==text{
					continue
				}
				//правим сообщение
				_, err := b.Edit(message, new_text)
				if err != nil {
					log.Println("Error edit status message:\n", err)
					send_to_admin("Ошибка изменения сообщения со статусом:\n" + err.Error())
				}
				text=new_text
			}
		}(message)
	}
}

//Обработчик кнопок "Локация свободна"
func hadle_rooms(button tb.InlineButton) {
	b.Handle(&button, func(c *tb.Callback) {
		//разбиваем текст на состовляющие по пробелу
		text := strings.Split(button.Text, " ")
		//получаем локацию
		location := get_location(text[0])
		//меняем ее статус
		location.Is_it_clear = true
		//записываем локацию
		chan_location <- location
		err := b.Respond(c, &tb.CallbackResponse{Text: "Готово"})
		if err != nil {
			log.Println("Respons: ", err)
			send_to_admin("Ошибка ответа по кнопке свободна:\n" + err.Error())
		}
	})
}

//Обработчик кнопок подсказок
func hadle_help(button tb.InlineButton) {
	b.Handle(&button, func(c *tb.Callback) {
		//получаем команду из названия группы и локацию из команды
		team := get_team(c.Message.Chat.Title)
		location := get_location(team.Сurrent_locastion)
		//собираем заново сообщение но уже без клавиатуры с подсказкой
		if location.With_video {
			var a tb.Video
			if location.Media_from_disk {
				a = tb.Video{File: tb.FromDisk(location.Media), Caption: location.Queston}
			}
			if location.Media_from_url {
				a = tb.Video{File: tb.FromURL(location.Media), Caption: location.Queston}
			}
			//изменяем сообщение убрав клавиатуру
			_, err := b.Edit(c.Message, &a)
			if err != nil {
				//Если ошибка "message is not modified" то значит на кнопку нажали 2 раза
				if strings.Contains(err.Error(), "message is not modified") {
					err = b.Respond(c, &tb.CallbackResponse{Text: "Подсказка уже использована"})
					if err != nil {
						log.Println("Respons: ", err)
						send_to_admin("Ошибка ответа по кнопке нет")
					}
					return
				}
				log.Println("Error edit message:\n", err)
				send_to_admin("Ошибка изменения сообщения:\n" + err.Error())
			}
		} else if location.With_photo {
			var a tb.Photo
			if location.Media_from_disk {
				a = tb.Photo{File: tb.FromDisk(location.Media), Caption: location.Queston}
			}
			if location.Media_from_url {
				a = tb.Photo{File: tb.FromURL(location.Media), Caption: location.Queston}
			}
			//изменяем сообщение убрав клавиатуру
			_, err := b.Edit(c.Message, &a)
			if err != nil {
				//Если ошибка "message is not modified" то значит на кнопку нажали 2 раза
				if strings.Contains(err.Error(), "message is not modified") {
					err = b.Respond(c, &tb.CallbackResponse{Text: "Подсказка уже использована"})
					if err != nil {
						log.Println("Respons: ", err)
						send_to_admin("Ошибка ответа по кнопке нет")
					}
					return
				}
				log.Println("Error edit message:\n", err)
				send_to_admin("Ошибка изменения сообщения:\n" + err.Error())
			}
		} else if location.With_audio {
			var a tb.Audio
			if location.Media_from_disk {
				a = tb.Audio{File: tb.FromDisk(location.Media), Caption: location.Queston}
			}
			if location.Media_from_url {
				a = tb.Audio{File: tb.FromURL(location.Media), Caption: location.Queston}
			}
			//изменяем сообщение убрав клавиатуру
			_, err := b.Edit(c.Message, &a)
			if err != nil {
				//Если ошибка "message is not modified" то значит на кнопку нажали 2 раза
				if strings.Contains(err.Error(), "message is not modified") {
					err = b.Respond(c, &tb.CallbackResponse{Text: "Подсказка уже использована"})
					if err != nil {
						log.Println("Respons: ", err)
						send_to_admin("Ошибка ответа по кнопке нет")
					}
					return
				}
				log.Println("Error edit message:\n", err)
				send_to_admin("Ошибка изменения сообщения:\n" + err.Error())
			}
		} else {
			//изменяем сообщение убрав клавиатуру
			_, err := b.Edit(c.Message, location.Queston)
			if err != nil {
				//Если ошибка "message is not modified" то значит на кнопку нажали 2 раза
				if strings.Contains(err.Error(), "message is not modified") {
					err = b.Respond(c, &tb.CallbackResponse{Text: "Подсказка уже использована"})
					if err != nil {
						log.Println("Respons: ", err)
						send_to_admin("Ошибка ответа по кнопке нет")
					}
					return
				}
				log.Println("Error edit message:\n", err)
				send_to_admin("Ошибка изменения сообщения:\n" + err.Error())
			}
		}
		//отправляем подсказку
		_, err := b.Send(c.Message.Chat, location.Help)
		if err != nil {
			log.Println("Error send help:\n", err)
			send_to_admin("Ошибка отправки подсказки:\n" + err.Error())
		}
		//добавляем команде штрафное время и записываем
		team.Time_added += config.Help_cost
		chan_team <- team
		err = b.Respond(c, &tb.CallbackResponse{})
		if err != nil {
			log.Println("Respons: ", err)
			send_to_admin("Ошибка ответа по кнопке подсказки:\n" + err.Error())
		}
	})
}

//Слушатеть кнопок "Финиш для команды"
func hadle_team_button(button_with_team tb.InlineButton) {
	b.Handle(&button_with_team, func(c *tb.Callback) {
		//разбиваем текст кнопки по пробелам и получаем название команды
		text := strings.Split(button_with_team.Text, " ")
		team := get_team(text[3])
		//если команда финишировала
		if team.Finished {
			err := b.Respond(c, &tb.CallbackResponse{Text: "Эта команда уже финишировала"})
			if err != nil {
				log.Println("Respons: ", err)
				send_to_admin("Ошибка ответа по кнопке финиш:\n" + err.Error())
			}
			return
		}
		//меняем файл команды и записываем его
		team.Finished = true
		team.Time_finish = time.Now()
		chan_team <- team
		err := b.Respond(c, &tb.CallbackResponse{Text: "Принято"})
		if err != nil {
			log.Println("Respons: ", err)
			send_to_admin("Ошибка ответа по кнопке финиша:\n" + err.Error())
		}
	})
}

//Слушатесь входящих сообщений
func hadle_messeges() {
	b.Handle(tb.OnText, func(m *tb.Message) {
		//если сообщение из группы админов то ничего не делать
		for _, admin_id := range config.Admins_ids {
			if m.Chat.ID == admin_id {
				return
			}
		}
		//получаем группу
		team := get_team(m.Chat.Title)
		//если текущая локация пустая то отправляем сообщение
		if team.Сurrent_locastion == "" {
			if len(team.Finished_locastions) == 0 {
				b.Send(m.Chat, "✋Терпение!\nИгра ещё не началась! 😉")
				return
			} else {
				b.Send(m.Chat, "🥲Все комнаты заняты!\n🙏🏼Пожалуйста, подождите!")
				return
			}
		}
		//получаем локацию
		location := get_location(team.Сurrent_locastion)
		//проверяем ответ
		if strings.ToLower(m.Text) == strings.ToLower(location.Answer) {
			b.Send(m.Chat, "🥳 Вы на верном пути!")
			//меняем файл команды
			team.Finished_locastions = append(team.Finished_locastions, team.Сurrent_locastion)
			//получаем новую локацию
			new_location := get_free_location_for_team(team)
			team.Сurrent_locastion = new_location.Name
			chan_team <- team
			if new_location.Name == "" {
				//если локация пустая, то отправляем сообщение и пытаемся получить новую
				_, err := b.Send(m.Chat, "🥲Все комнаты заняты!\n🙏🏼Пожалуйста, подождите!")
				if err != nil {
					log.Println("Error send busy room message:\n", err)
					send_to_admin("Ошибка отправки сообщение о занятости всех комнат:\n" + err.Error())
				}
				for new_location.Name == "" {
					//Если новая локация пустая то ждем и пытаемся снова
					time.Sleep(10 * time.Second)
					new_location = get_free_location_for_team(team)
					team.Сurrent_locastion = new_location.Name
					chan_team <- team
				}
			}
			//меняем файл локации
			new_location.Is_it_clear = false
			//записываем измененное состояние локации и команды
			chan_location <- new_location
			team.Сurrent_locastion = new_location.Name
			chan_team <- team
			//отправляем новое задание
			send_to_team(new_location, team)
		} else {
			//если ответ не верный
			b.Send(m.Chat, "👎🏻 К сожалению, это неправильный ответ!\n🕵️Попробуйте ещё раз!")
		}
	})
}

//Проверяет и создает пользователя, файл и папки
func adduser(dir string, user int64, nick string, FirstName string, LastName string) {
	//создаем папки если их нет
	if _, err := os.Stat(config.Path_to_files + currentTime); os.IsNotExist(err) {
		os.Mkdir(config.Path_to_files+currentTime, 0755)
	}
	if _, err := os.Stat(config.Path_to_files + currentTime + "/" + strconv.Itoa(int(user)) + ".yaml"); os.IsNotExist(err) {
		//создаем файл если его нет
		f, err := os.Create(config.Path_to_files + currentTime + "/" + strconv.Itoa(int(user)) + ".yaml")
		f.Close()
		if err != nil {
			log.Println("Can't create user file: ", err)
			send_to_admin("Ошибка создания файла пользователя" + strconv.Itoa(int(user)) + " @"+nick+" "+ FirstName +" "+LastName+":\n" + err.Error())
		}
	} else if err == nil {
		return
	}
	//наполняем данными пользователя
	var a User
	a.ID = user
	a.Nick = nick
	a.FirstName = FirstName
	a.LastName = LastName
	a.Number = get_number()
	//записываем пользователя
	chan_users <- a
	se := new(tb.Chat)
	se.ID = user
	//отправляем номер для участия в лотерее
	m, err := b.Send(se, "🎁 Ваш номер в розыгрыше:  "+strconv.Itoa(a.Number)+"\nЛотерея пройдёт в конце нашего вечера!")
	if err != nil {
		log.Println("Error send number to user: ", err)
		send_to_admin("Ошибка отправки номера:\n" + err.Error())
	}
	//закрепляем сообщение в чате
	err = b.Pin(m)
	if err != nil {
		log.Println("Error pin message\n", err)
		send_to_admin("Ошибка закрепления сообщения :\n" + err.Error())
	}
}

//Возвращает свободную локацию для данной группы
func get_free_location_for_team(team Team) Location {
	//получаем список локаций
	for location_name, _ := range config.Locations {
		location := get_location(location_name)
		//проверяем чистая ли она
		if !location.Is_it_clear {
			continue
		}
		//проверяыем была ли там эта команда
		if was_in_this_location(team, location.Name) {
			continue
		}
		return location
	}
	//если количество пройденным локаций равна количествву всех локаций то возвращаем финальную
	if len(team.Finished_locastions) == len(config.Locations) {
		return config.Final_location
	}
	//если ничего не получилось возвращаем пустую локацию
	var l Location
	return l
}

//Возвращает номер для участия в лотерее
func get_number() int {
	//проверяем есть ли файл и создаем его
	if _, err := os.Stat(config.Path_to_files + currentTime + "/" + "number"); os.IsNotExist(err) {
		f, err := os.Create(config.Path_to_files + currentTime + "/" + "number")
		ioutil.WriteFile(config.Path_to_files+currentTime+"/"+"number", []byte("1"), 0666)
		f.Close()
		if err != nil {
			log.Println("Can't create number file: ", err)
			send_to_admin("Ошибка создания файла c номером" + ":\n" + err.Error())
		}
	}
	//читаем файл
	file, err := ioutil.ReadFile(config.Path_to_files + currentTime + "/" + "number")
	if err != nil {
		log.Println("Can't read number file: ", err)
		send_to_admin("Ошибка чтения файла c номером" + ":\n" + err.Error())
	}
	number, err := strconv.Atoi(string(file))
	if err != nil {
		log.Println("Can't convert number: ", err)
		send_to_admin("Ошибка конвертации номера" + ":\n" + err.Error())
	}
	//увеличиваем
	content := []byte(strconv.Itoa(number + 1))
	//записываем
	err = ioutil.WriteFile(config.Path_to_files+currentTime+"/"+"number", content, 0666)
	if err != nil {
		log.Println("Can't write number file: ", err)
		send_to_admin("Ошибка записи файла c номером" + ":\n" + err.Error())
	}
	//возвращаем
	return number
}

//Возвращает текст со статусом всех команд и занятых локаций
func get_status() string {
	//готовим сообщение для отправки
	var message string
	//переменные для сверки количества занятых локаций и количество локаций на которых команды
	var tl, ll int
	for _, team := range config.Teams {
		t := get_team(team.Name)
		if t.Сurrent_locastion != "" {
			message += "Команда " + t.Name + " в " + t.Сurrent_locastion + "\n"
			tl += 1
		} else {
			message += "Команда " + t.Name + " ждет свою локацию\n"
		}
	}
	for _, location := range config.Locations {
		l := get_location(location.Name)
		if !l.Is_it_clear {
			message += location.Name + " занята\n"
			ll += 1
		}
	}
	//если количество занятых локаций и количество команд не ждуших свои локации одинковое то ✅ иначе ❌
	if tl == ll {
		message += "✅"
	} else {
		message += "❌"
	}
	return message
}

//Возвращает одну локацию
func get_location(name string) Location {
	//чиитаем файл
	yamlFile, err := ioutil.ReadFile(config.Path_to_files + currentTime + "/locations/" + name + ".yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
		send_to_admin("Ошибка чтения файла локации" + name + ":\n" + err.Error())
	}
	//преобразуем
	var l Location
	err = yaml.Unmarshal(yamlFile, &l)
	if err != nil {
		log.Printf("Unmarshal: %v", err)
		send_to_admin("Ошибка парсинга локации" + name + ":\n" + err.Error())
	}
	//возвращаем
	return l
}

//Возвращает одного пользователя
func get_user(file_name string) User {
	//читаем
	yamlFile, err := ioutil.ReadFile(config.Path_to_files + currentTime + "/" + file_name + ".yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
		send_to_admin("Ошибка чтения файла пользователя" + file_name + ":\n" + err.Error())
	}
	//преобразуем
	var u User
	err = yaml.Unmarshal(yamlFile, &u)
	if err != nil {
		log.Printf("Unmarshal: %v", err)
		send_to_admin("Ошибка парсинга пользователя" + file_name + ":\n" + err.Error())
	}
	//возвращаем
	return u
}

//Возвращает одну группу
func get_team(file_name string) Team {
	//читаем
	yamlFile, err := ioutil.ReadFile(config.Path_to_files + currentTime + "/teams/" + file_name + ".yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
		send_to_admin("Ошибка чтения файла группы" + file_name + ":\n" + err.Error())
	}
	//преобразуем
	var t Team
	err = yaml.Unmarshal(yamlFile, &t)
	if err != nil {
		log.Printf("Unmarshal: %v", err)
		send_to_admin("Ошибка парсинга группы" + file_name + ":\n" + err.Error())
	}
	//возвращаем
	return t
}

//Запись пользователя в файл
func write_user() {
	for {
		//слушаем канал
		u, ok := <-chan_users
		if ok == false {
			//выходим из цикла
			log.Println(ok, "<-- loop broke!")
			break // exit break loop
		} else {
			//преобразуем
			content, err := yaml.Marshal(u)
			if err != nil {
				log.Printf("Marshal: %v", err)
				send_to_admin("Ошибка превращения пользователя " + strconv.Itoa(int(u.ID)) + " в структуру для записи в файл:\n" + err.Error())
			}
			//записываем
			if _, err := os.Stat(config.Path_to_files+currentTime+"/"+strconv.Itoa(int(u.ID))+".yaml"); os.IsNotExist(err) {
				f, err := os.Create(config.Path_to_files+currentTime+"/"+strconv.Itoa(int(u.ID))+".yaml")
				f.Close()
				if err != nil {
					log.Println("Can't create user file: ", err)
					send_to_admin("Ошибка создания файла c пользователем" + ":\n" + err.Error())
				}
			}
			err = ioutil.WriteFile(config.Path_to_files+currentTime+"/"+strconv.Itoa(int(u.ID))+".yaml", content, 0666)
			if err != nil {
				log.Println("WriteFile: ", err)
				send_to_admin("Ошибка записи пользователя " + strconv.Itoa(int(u.ID)) + " в файл:\n" + err.Error())
			}
		}
	}
}

//Запись группы в файл
func write_team() {
	for {
		//слушаем канал
		t, ok := <-chan_team
		if ok == false {
			//выходим из цикла
			log.Println(ok, "<-- loop broke!")
			break // exit break loop
		} else {
			//преобразуем
			content, err := yaml.Marshal(t)
			if err != nil {
				log.Printf("Marshal: %v", err)
				send_to_admin("Ошибка превращения группы " + t.Name + " в структуру для записи в файл:\n" + err.Error())
			}
			//записываем
			if _, err := os.Stat(config.Path_to_files+currentTime+"/teams/"+t.Name+".yaml"); os.IsNotExist(err) {
				f, err := os.Create(config.Path_to_files+currentTime+"/teams/"+t.Name+".yaml")
				f.Close()
				if err != nil {
					log.Println("Can't create team file: ", err)
					send_to_admin("Ошибка создания файла c группой" + ":\n" + err.Error())
				}
			}
			err = ioutil.WriteFile(config.Path_to_files+currentTime+"/teams/"+t.Name+".yaml", content, 0666)
			if err != nil {
				log.Println("WriteFile: ", err)
				send_to_admin("Ошибка записи группы " + t.Name + " в файл:\n" + err.Error())

			}
		}
	}
}

//Запись локации в файл
func write_location() {
	for {
		//слушаем канал
		l, ok := <-chan_location
		if ok == false {
			//выходим из цикла
			log.Println(ok, "<-- loop broke!")
			break // exit break loop
		} else {
			//преобразуем
			content, err := yaml.Marshal(l)
			if err != nil {
				log.Printf("Marshal: %v", err)
				send_to_admin("Ошибка превращения локации в структуру для записи в файл:\n" + err.Error())
			}
			//записываем в файл
			if _, err := os.Stat(config.Path_to_files+currentTime+"/locations/"+l.Name+".yaml"); os.IsNotExist(err) {
				f, err := os.Create(config.Path_to_files+currentTime+"/locations/"+l.Name+".yaml")
				f.Close()
				if err != nil {
					log.Println("Can't create location file: ", err)
					send_to_admin("Ошибка создания файла c локацией" + ":\n" + err.Error())
				}
			}
			err = ioutil.WriteFile(config.Path_to_files+currentTime+"/locations/"+l.Name+".yaml", content, 0666)
			if err != nil {
				log.Println("WriteFile: ", err)
				send_to_admin("Ошибка записи локации в файл:\n" + err.Error())
			}
		}
	}
}

//Записывает время начала
func write_begin_time() {
	//преобразуем
	var t Time
	t.Begin = time.Now()
	content, err := yaml.Marshal(t)
	if err != nil {
		log.Printf("Marshal: %v", err)
		send_to_admin("Ошибка превращения времени начала в структуру для записи в файл:\n" + err.Error())
	}
	if _, err := os.Stat(config.Path_to_files + currentTime + "/game_begin_time.yaml"); os.IsNotExist(err) {
		//создаем файл если его нет
		f, err := os.Create(config.Path_to_files + currentTime + "/game_begin_time.yaml")
		f.Close()
		if err != nil {
			log.Println("Can't create time file: ", err)
			send_to_admin("Ошибка создания файла cо временем старта:\n" + err.Error())
		}
	}
	//записываем в файл
	err = ioutil.WriteFile(config.Path_to_files+currentTime+"/game_begin_time.yaml", content, 0666)
	if err != nil {
		log.Println("WriteFile: ", err)
		send_to_admin("Ошибка записи времени в файл:\n" + err.Error())
	}
}

//Проверяет была ли данная группа в данной локации
func was_in_this_location(team Team, l string) bool {
	for _, location := range team.Finished_locastions {
		if location == l {
			return true
		}
	}
	return false
}

//Отправляет сообщение в админский чат
func send_to_admin(text string) {
	//если сообщение длинее 4095 символов то его надо разбить на несколько
	if len(text) > 4095 {
		for i := 0; i < len(text); i += 4095 {
			for _, admin_id := range config.Admins_ids {
				//готовим чат
				se := new(tb.Chat)
				se.ID = admin_id
				//проверяем выходим ли мы за пределы длинны
				if i+4095 > len(text) {
					//если выходим отправляем весь оставшийся текст
					_, err := b.Send(se, text[i:len(text)])
					if err != nil {
						log.Println("Error send error to admin: ", err)
						send_to_admin("Ошибка отправки ошибки:\n" + err.Error())
					}
				} else {
					//если не выходим то отправляем 4095 символом
					_, err := b.Send(se, text[i:i+4095])
					if err != nil {
						log.Println("Error send error to admin: ", err)
						send_to_admin("Ошибка отправки ошибки:\n" + err.Error())
					}
				}
			}
		}
	} else {
		for _, admin_id := range config.Admins_ids {
			//готовим чат
			se := new(tb.Chat)
			se.ID = admin_id
			//отправляем весь текст сразу
			_, err := b.Send(se, text)
			if err != nil {
				log.Println("Error send error to admin: ", err)
				send_to_admin("Ошибка отправки ошибки:\n" + err.Error())
			}
		}
	}
}

//Отправляет сообщение в группу
func send_to_team(location Location, team Team) {
	//готовим чат
	se := new(tb.Chat)
	se.ID = team.Chat_id
	//создаем кнопки помощи
	help_button := tb.InlineButton{
		Unique: "Help" + strconv.Itoa(int(team.Chat_id)),
		Text:   "Подсказку?",
	}
	hadle_help(help_button)
	//собираем сообщение
	if location.With_video {
		var a tb.Video
		if location.Media_from_disk {
			a = tb.Video{File: tb.FromDisk(location.Media), Caption: location.Queston}
		}
		if location.Media_from_url {
			a = tb.Video{File: tb.FromURL(location.Media), Caption: location.Queston}
		}
		//отправляем вопрос с видео
		_, err := b.Send(se, &a, &tb.ReplyMarkup{InlineKeyboard: [][]tb.InlineButton{[]tb.InlineButton{help_button}}})
		if err != nil {
			log.Println("Send to: "+strconv.Itoa(int(se.ID)), ":\n", err)
			send_to_admin("Ошибка отправки:\n" + err.Error() + "\nКому:" + strconv.Itoa(int(se.ID)))
		}
	} else if location.With_photo {
		var a tb.Photo
		if location.Media_from_disk {
			a = tb.Photo{File: tb.FromDisk(location.Media), Caption: location.Queston}
		}
		if location.Media_from_url {
			a = tb.Photo{File: tb.FromURL(location.Media), Caption: location.Queston}
		}
		//отправляем вопрос с фото
		_, err := b.Send(se, &a, &tb.ReplyMarkup{InlineKeyboard: [][]tb.InlineButton{[]tb.InlineButton{help_button}}})
		if err != nil {
			log.Println("Send to: "+strconv.Itoa(int(se.ID)), ":\n", err)
			send_to_admin("Ошибка отправки:\n" + err.Error() + "\nКому:" + strconv.Itoa(int(se.ID)))
		}
	} else if location.With_audio {
		var a tb.Audio
		if location.Media_from_disk {
			a = tb.Audio{File: tb.FromDisk(location.Media), Caption: location.Queston}
		}
		if location.Media_from_url {
			a = tb.Audio{File: tb.FromURL(location.Media), Caption: location.Queston}
		}
		//отправляем вопрос с аудио
		_, err := b.Send(se, &a, &tb.ReplyMarkup{InlineKeyboard: [][]tb.InlineButton{[]tb.InlineButton{help_button}}})
		if err != nil {
			log.Println("Send to: "+strconv.Itoa(int(se.ID)), ":\n", err)
			send_to_admin("Ошибка отправки:\n" + err.Error() + "\nКому:" + strconv.Itoa(int(se.ID)))
		}
	} else {
		//отправляем вопрос
		_, err := b.Send(se, location.Queston, &tb.ReplyMarkup{InlineKeyboard: [][]tb.InlineButton{[]tb.InlineButton{help_button}}})
		if err != nil {
			log.Println("Send to: "+strconv.Itoa(int(se.ID)), ":\n", err)
			send_to_admin("Ошибка отправки:\n" + err.Error() + "\nКому:" + strconv.Itoa(int(se.ID)))
		}
	}
}
