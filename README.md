Этот телеграмм бот создаст вам квест игру по определенным к config файле локациям

После настройки и запуска бот будет ждать сообщения /start чтобы создать локальную папку с настройками локаций, комманд, и там же будут сохранены профили пользовотелей

При получении комманды /start от пользователя бот присваивает ему уникальный идентификатор для участия в лотерее и спрашивает будет ли пользователь участвовать в квесте

В случае положительного ответа бот сам равномерно распределяет пользователей по коммандам (группам) описанных в конфиг файле

При получении комманды /start от администратора(ов) бот присылает клавиатуру с кнопками:

1. "начать" - нчаать квест (то есть начать отправлять задания)
2. "Локация свободна" - меняет файл локации и служить для того чтобы комманды не пересекались
3. "Финиш для команды" - фиксирует время завершения
4. "Результаты" - выводить за сколько комманда справилась со всеми заданиями, сколько у каждой комманды штрафного времени и итог

Так же после начала квеста бот пришлет сообщение со статусом всех комманд + перечислит все занятые локации. Данное сообщение будет обновляться автоматически один раз в минуту если статусы комманд/локаций изменять. Требуется для контроля освобождения локаций

В каждом вопросе бота есть подсказка, в случае ее использования бот так же изменит сообщение с вопросом и уберет кнопку подсказки, что требуется для предотвращения множественного сипользования подсказок

Все ошибки происходящие в боте будут отправляться так же в админский чат

Для корректной работы требуется заранее назначить бота администратором групп и включить просмотр истории новоприбывшим пользователям

В случае не надобности распределенния пользователей по группам (участики уже распределенны и знают друг друга) просто добавте бота в чат комманды начинать диалог в чате с ботом не требуется (однако в конфиг файле все равно нужно внести данные комманд)

Обязательные параметры конфиг файла:

    Path_to_files - где хранить все файлы
    Telegram_token - токент телеграмм бота полученный от @BotFather
    Welcome - текст приветсвия (что прислать человеку после комманды /start)
    Users_limit - максимальное количество пользователей для участия в квесте
    Will_play_question - текст вопроса будет ли пользователь принимать участие в квесте
    Rules - правила квеста (отправляются в группу и закрепляются)
    Help_cost - стоимость подсказки в минутах
    Admins_ids - массив id чатов или пользователей админов/ведущих (можно получить тут @getmyid_bot)
    Teams - массив комманд со следующими значениями:
    Chat_id - идентификатор чата (можно получить тут @getmyid_bot)
    Name - имя чата (должно совпадать с реальным именем группы в телеграмме)
    URL - ссылка для приглашения новых пользователей
    Locations - словать локаций со следующими обезутельными полями:
    Is_it_clear - готова ли локация принимать участников
    Queston - загадка/вопрос на который вользователи должны найти ответ
    Answer - правильный ответ
    Help - подсказка
    Name - название локации
  
Также в конфиг файле есть не обезательные поля:

    Welcome_with_photo - если к тексту приветсвия требуется приложить картинку
    Welcome_with_video - если к тексту приветсвия требуется приложить видео
    Media_from_disk/Media_from_url - от куда брать требуемый файл (диск или ссылка)
    Media - ссылка/путь на файл
    В локациях также есть не обязательные параметры:
    With_photo/With_video/With_audio - если треутся к загадке приложить фото/видео/аудио
    Media_from_disk/Media_from_url - от куда брать требуемый файл (диск или ссылка)
    Media - ссылка/путь на файл фото/видео/аудио
  
