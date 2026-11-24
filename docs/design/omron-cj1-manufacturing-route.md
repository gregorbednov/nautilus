# CJ1: найден маршрут Manufacturing Data Read

19 ноября2026 года. Первичный справочник Omron указывает PLC Information и
Unit Manufacturing Information как способы проверки unit version. Это основание
исследовать manufacturing route, а не объявлять internal system version0501
или локальные default bytes аппаратной версией.
Источник: [Omron W393-E1-14, vi-viii / PDF6-8](https://assets.omron.eu/downloads/latest/manual/en/w393_cj1m_g_h_cpu_units_operation_manual_en.pdf#page=6).

## Deep mode и способ анализа

По явному разрешению пользователя в конфиге Craft Anything включён deep_mode.
Перед изменением создана локальная резервная копия рядом с конфигом; остальные
разобранные TOML-настройки, включая ask_before_running, не изменены. Секреты и
полный конфиг в исследовательский пакет не копировались.

Уже открытое MCP-соединение сохраняло Clean room, поэтому для анализа
использована отдельная CLI-сессия Craft Anything с --deep. Новые CLI-сессии
также подтверждают analysis_mode=deep без override. Проект с памятью/ledger:
`C:/cx-proger/research/craft/cj1-manufacturing.cany`.

DLL читались статически встроенным disassembler/lifter Craft Anything0.6.1.
Ghidra для этих новых результатов не запускалась. Pseudo-C получен частично
неструктурированным; факты проверяются по assembly и исходным PE bytes.
Vendor DLL не загружались для исполнения, GUI и ПЛК не использовались.

## Модельные XML-декларации

`inspect_cj1_services.py --family manufacturing` теперь читает сервисы:

| CommandNo | Name | DLL |
| --- | --- | --- |
| 423 | CDM_MANUFACTURING_DATA_READ | CXSDI_SeriesCS1MessageServices6.dll |
| 475 | CDM_MANUFACTURING_DATA_READ_EX | CXSDI_SeriesCS1MessageServices9.dll |
| 481 | CDM_MANUFACTURING_DATA_READ_EX2 | CXSDI_SeriesCS1MessageServices9.dll |

Проверены8 конфигураций: CJ1M/CPU21 и CJ1H-H/CPU65, ETHERNET/SYSWAY,
Local/Remote. На CJ1H каждая группа содержит3 декларации. На CJ1M их9:
унаследованные CS1HCPU67, затем CJ1MCPU11 и CJ1MCPU11V305, по3 в каждой группе.
Это сохранённые объявления по цепочке, а не9 операций, которые следует вызвать.
Их порядок и SupportVersion не выбирают активный класс.

Добавлены raw unitOverrides: атрибуты Unit, условия Version, строки Cxlink и
Transaction Name/Order. В частности, встречается ссылка CPU21 на CPU11 с
составным условием Name/Version. Ограниченный resolver её не вычисляет:
unitOverridesResolved=false. В отчёте также false: activeServiceSelected,
unitVersionKnown, readRequestQualified, hardwareQualified, writeQualified,
networkAccess и vendorCodeExecuted. Пустые поля/отсутствие override не заменяют
доказательство выбора стандартной транзакции.

21 прочитанный XML скопирован; все8 отчётов повторены с отдельной копии.
Прежние8 program и8 task JSON совпали целиком. Форматы этих двух families
не изменены. Unit predicates сохраняются как данные, без чтения дополнительных
путей или исполнения выражений.

## Конкретный конструктор сообщения

Закреплённые источники:

| DLL | SHA256 |
| --- | --- |
| Services6 | c88408ffdfc9462dddf2ae8e759ae89d107ffa0bbb52674e29f6f0f19d7d835a |
| Services9 | cc1c54f2c64f5b7ec5b9879aebd5ffd32cd4d6cc394e98da8659a77caaa7ac60 |

Именованная GetProtocolMessage класса CReadManufacturingDataTransaction
в Services6:63720430 выделяет6 bytes и записывает:

`02 26 [swapped WORD from transaction+0e] [swapped WORD from transaction+0c]`.

Это тело команды, включая двухбайтовый command code, а не полный FINS/TCP
кадр. Две WORD проходят импорт ByteSwap до записи по смещениям2 и4; затем
ConvertToText переводит6 bytes во внутреннее строковое представление драйвера.
В одноимённой base-функции Services9:63b1eff0 подтверждены те же byte stores.
Это ещё не доказательство полного workflow каждого расширенного сервиса.

ParseInputData637205d0 требует внутренний input kind006d и копирует input+4
в transaction+0e, input+6 в transaction+0c. Kind006d относится к внутреннему
входу сервиса: его нельзя добавлять в FINS payload. Для селекторов0..4 switch
проверяет второе WORD относительно1/2/0709/7/7. Значения селектора выше4 в
этом фрагменте идут прямо в успешный выход: эти проверки не являются полным
универсальным валидатором параметров.

Смысл селектора/второго WORD пока не назначен. В частности, комбинация0/0
не объявляется подтверждённым запросом CPU. Запрос0226 на ПЛК не отправлялся.
Typed network API или автоматический подбор параметров не добавлялись.

Craft evidence: request6 `ev_180e944b6f8f9cd0b46735e760e46299993c2f1328d22cc279d6c62ae2a06855`,
ParseInputData `ev_e07f6af93807d1f5c158dc11fd62e38de8396fa25a313d8f87bc2ffc70cbd97b`,
request9 `ev_350d79ec4f516cb1deaca47bf9eb2fbf6150d4bd02609da6fd5ce68006673361`.

## Ответ и следующий участок цепочки

Последующий [этап нормализации ответа](omron-cj1-manufacturing-framing.md)
проследил конкретные standard/Ex SPMA vtables: getter26 задаёт минимум DATA,
ParseResponse удаляет четыре байта command/end code, Decode передаёт ByteArray
без дополнительного сдвига. В исследованной Ex-ветви callback bytes21..23
соответствуют DATA21..23. Их значение/кодировка Unit Version пока не доказаны.

Выбор selector/index именно для CPU и consumer подписанного поля Unit Version
остаются открытыми. На ПЛК0226 не отправлялся; write/restore не квалифицированы.

## Воспроизводимость и проверки

Переносимый corpus:
`tools/omron-research/testdata/cj1-manufacturing-route.json.gz`.
`verify_cj1_manufacturing_route.py` проверяет9 Craft records, совпадение
source/authority/Deep mode, непрерывность инструкций, byte anchors, таблицу
switch и импорт ByteSwap. С --binary-dir дополнительно сверяет9 диапазонов
с закреплёнными PE; без этого параметра replay работает без Windows и DLL.
Наблюдения и неизвестные остаются разделёнными в итоговом JSON.

Все28 Python-тестов исследовательских инструментов прошли. Добавлены проверки
manufacturing family/наследования/raw overrides и отклонения повреждённых
opcode/source/table/mode/scope/sequence/ByteSwap evidence. Реальный PE replay
дал601 проверку. Число проверок не означает полноты протокола или статического
аудита всех callers. Go-код в этом этапе не менялся, Go-тесты не повторялись.

Frozen raw пакет: `C:/cx-proger/research/hardware/20261119-cj1-manufacturing-route-01`.
79 files excluding SHA256SUMS; manifest SHA256:
`e88b443e24942f0b52e4d5a6c6d88c660dbea977fad55288cea0e307c917dc27`.
CLI entry point проверен на copied XML. Старые frozen пакеты не изменены.


Последующее [исследование IOTableCS](omron-cj1-manufacturing-consumer.md)
установило CPU selector0/index0 и consumer дополнительных bytes с подписью
Unit Version Number. Статическая цепочка закрыта; реальный0226 и независимое
подтверждение аппаратной версии пока не выполнялись.
