# CJ1 manufacturing response framing — 2026-11-19

Продолжение [manufacturing route](omron-cj1-manufacturing-route.md).
Статически установлена нормализация ответа в двух конкретных ветвях:
Services6 standard SPMA и Services9 Ex SPMA. Запросов к ПЛК и выполнения
vendor DLL в этом этапе не было. Выбор CPU и декодирование Unit Version
остаются открытыми; поддержка записи/восстановления не квалифицирована.

## Подтверждённая цепочка

| Участок | Services6 | Services9 Ex |
| --- | --- | --- |
| Constructor SPMA → vtable | 6371bd80 → 6373a6e0 | 63b18e70 → 63b40f6c |
| Service slot64 ParseResponse | 6370bb10 | 63b024b0 |
| Transaction constructor → vtable | 63720400 → 6373a69c | 63b1ee90 → 63b40994 |
| Transaction slot0c GetExpectDataBytes | 63720740 | 63b1f300 |
| Common ReturnProtocolMessage | 6370c3e0 | 63b02d80 |
| Transaction slot14 Decode | 637206f0 | 63b1f2b0 |
| Transaction slot34 CreateCallbackData | 63720660 | 63b1eec0 |

GetExpectDataBytes возвращает26. Common ReturnProtocolMessage передаёт это
значение третьим аргументом ParseResponse. Для успешного end code ParseResponse
требует длину тела не меньше `26+4`, затем вызывает
`ByteArray.SetData(pointer+4, length-4)`. Первые четыре байта — command code
и end code. Это минимум26 байт DATA, а не точная длина ответа.

Полученный ByteArray передаётся в Decode, затем без дополнительного сдвига
в callback. В исследованной Ex-ветви bytes21..23 callback source поэтому
соответствуют смещениям21..23 DATA. Callback переносит их в offsets23..25
внутреннего результата типа2cc длиной26. Значение и кодировка этих байтов
не установлены; гипотеза их связи с Unit Version остаётся `testing`.
Промежуточные поля внутренних структур нельзя выдавать за сетевой формат.

Craft project `C:/cx-proger/research/craft/cj1-manufacturing.cany`, finding
`fnd_3` связывает vtable, handlers и byte operations. Исходные SHA256 Services6/9
сохранены в переносимом corpus и [предыдущем отчёте](omron-cj1-manufacturing-route.md).
Ghidra12.1.4 независимо дизассемблировала шесть выбранных функций Services6;
адреса и мнемоники согласуются с Craft. Типы и соглашения о вызовах в
псевдокоде частично восстановлены: вывод опирается на инструкции и vtable.

## Реализация и воспроизводимость

`tools/omron-research/inspect_cj1_manufacturing_response.py` — переносимый
офлайн-анализатор сохранённого command response body:
`02 26 [end code:2] [DATA]`. Полный FINS/TCP кадр он не разбирает: привязка
адресов, SID, модели и запроса должна проверяться отдельно. Проверяет command,
ненулевой end code и минимальную длину; выдаёт SHA256 и исходные bytes21..23
без версии CPU. Любой ненулевой end code сохраняется в ошибке, включая status
bits: native-маскирование этих битов намеренно не перенесено. Ограничение
1004 байта относится к офлайн-инструменту, не к максимуму native-протокола.

`verify_cj1_manufacturing_framing.py` воспроизводит18 Craft records из
`testdata/cj1-manufacturing-framing.json.gz`, проверяет vtable slots,
инструкции нормализации и callback, источник/режим/границы доказательств,
а также адреса и мнемоники Ghidra. В recursive disassembly возможны пропуски
недостижимых блоков; проверяются порядок и отсутствие перекрытий. С
`--binary-dir` каждый сохранённый диапазон инструкции/таблицы сверяется
с PE после проверки полного SHA256 DLL. Без DLL replay работает офлайн,
в том числе на Linux; это проверка сохранённого corpus.

```text
python3 -B -m unittest discover -s tools/omron-research -p 'test_*.py'
python3 -B tools/omron-research/verify_cj1_manufacturing_framing.py
python3 -B tools/omron-research/verify_cj1_manufacturing_framing.py --binary-dir C:/Windows/SysWOW64
python3 -B tools/omron-research/inspect_cj1_manufacturing_response.py retained-body.bin
```

Все34 Python-теста прошли, включая повреждение vtable/минимума/сдвига/
callback/source/scope/Ghidra и некорректные ответы. С реальными PE:
3576 проверок,872 диапазона инструкций/таблиц. Количество проверок не
является мерой полноты протокола. Go-код не менялся, Go-тесты не повторялись.
Синтетические ответы в тестах не выдаются за аппаратные наблюдения.

Frozen raw пакет:
`C:/cx-proger/research/hardware/20261119-cj1-manufacturing-framing-01`.
51 файл без SHA256SUMS; SHA256 манифеста:
`f94a84bd193d5d2e75b3ec0b4faa415e89cd2a69c1ab5240fd17f45ebde0ea19`.
SHA256 переносимого gzip corpus:
`f150ec960c45682b79ff3870394c0635d76cc0f91151d8be0f01f644838507fc`.
Предыдущий frozen route-пакет не менялся.

## Следующие проверки

Проследить caller, который формирует selector/index именно для CPU, затем
consumer результата до подписанного поля Unit Version. Не отправлять
предполагаемую комбинацию0/0. Отдельная сохранённая зацепка:
CX-Pdata `IsExtendProtectionSupportedUnitVersion`121b1dfe импортируется
CX-P.exe; происхождение аргументов этого predicate пока не прослежено.
Порог поддержки в нём не устанавливает реальную версию или защиту CPU.

После доказательства выбора CPU подготовить ограниченное read-only чтение
с проверкой фактической модели и независимым подтверждением версии.
Финализация и полноценная запись/восстановление остаются отдельными этапами.


Продолжение: [CPU selector и consumer Unit Version](omron-cj1-manufacturing-consumer.md)
закрывает статическую связь selector0/index0 и поля Unit Version Number.
Следующим остаётся аппаратное read-only подтверждение; исторические неизвестные
выше относятся к моменту framing-этапа. Frozen framing-пакет не менялся.
