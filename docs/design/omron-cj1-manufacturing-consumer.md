# CJ1: CPU selector и consumer Unit Version — 2026-11-19

Статически закрыты два вопроса из
[этапа framing](omron-cj1-manufacturing-framing.md): selector/index для CPU
и назначение дополнительных байтов в исследованном Ex475 consumer.
Это ещё не измерение реальной версии CPU. Сети, запуска/регистрации OCX,
установки компонентов и записи в ПЛК в этом этапе не было.

## Источник

Из установочного `C:/cx/CXONE v4.40/CX Common Tools/PLCTool.cab`
через `expand.exe -F:exact-member` извлечён только
`iotablecs.ocx1.BC6EEA9C_E7D9_40CF_997B_7C98B9CAEF9E`,1724416 bytes.
SHA256 CAB: `e86c8f2e8554453a8d2e5cf022556b2518ac2e41721d7a56452aa7f0b7fe4ad4`.
SHA256 OCX: `b6a49acc3fc0531ddcbad97312d9ac14e12b6a56569fa2d5adf0193ef87babe5`.
Ресурсы называют продукт `OMRON IOTable Component`; RTTI содержит
`CManufactInfo`, `CN1ManufacturingInfo`, `CUnitVersion`.
Это исследованный файл установщика, а не доказательство его загрузки
текущим CX-Programmer. Версию из другого установленного экземпляра
нельзя молча приравнивать этому build.

Разбор: встроенные статические инструменты Craft Anything0.6.1 и Ghidra12.1.4.
Для неподхваченных Craft функций использованы адреса, проверенные по
инструкциям Ghidra и исходным PE bytes. Неполные типы псевдокода не заменяют
проверку ABI/вызовов. Поиски с отрицательным результатом и неуспешные запросы
сохранены как история поиска, а не как доказательство отсутствия функций.

## CPU selector

В RT_STRING2059/2057 ID `80a1` означает
`Manufacturing Information for CPU Unit`. AFX_MSGMAP_ENTRY по `100f7a40`
содержит `WM_COMMAND,0,80a1,80a1,12,100791be`. Обработчик `100791be`
передаёт0 в `100791d3`; тот передаёт selector в constructor `100919ba`
класса `CN1ManufacturingInfo`.

Инициализация `10091c04` вызывает `10091eb5(0)`. Последний создаёт
параметр kind006d с четырьмя байтами содержимого, копирует selector в первое
WORD и записывает0 во второе WORD для selector0 (также для1 и5).
Иные значения могут использовать rack/slot-подобную пару; её полная
семантика этим этапом не квалифицирована.

Диалог сначала вызывает сервис481 и при соответствующей ошибке475.
Отдельный `CManufactInfo::` путь `1006e9da` использует непосредственно475,
аналогично строит два WORD и получает callback `1006eada`.
`1006e97f` сначала читает selector0, затем при необходимости выбранный
ненулевой selector. Через уже сохранённую
[цепочку Services9](omron-cj1-manufacturing-route.md) для475 получается
кандидат command body **`02 26 00 00 00 00`**.
Это статическое основание для ограниченного CPU read, а не перебор параметров.
Внутренние callback layouts475 и481 различаются; их нельзя смешивать.

Craft finding `fnd_4`, project
`C:/cx-proger/research/craft/cj1-manufacturing.cany`.

## Что означают дополнительные bytes

Сохранённый Services9 Ex callback переносит DATA21..23 в offsets23..25
внутреннего26-байтового результата. `CManufactInfo` handler `1006eb70`
получает содержимое результата после двух четырёхбайтовых заголовков,
читает эти три байта без арифметической перекодировки и сохраняет три int.
`1006ed50 → 100cb864` передаёт их в `CUnitVersion`.

Независимая UI-ветвь475 `10092484` формирует из result23/24 строку
**`%X.%X`** в поле dialog+90, а из result25 — **`%X`** в dialog+94.
Печатный consumer `10093e46` выводит dialog+90 рядом с RT_STRING389
`Unit Version Number`. RT_STRING390 — `Unit Version Revision Number`.
Связь дополнительных bytes с Unit Version теперь подтверждена статически.
В частности, `12 0a 10` отображается как `12.A` и revision `10`.
Нельзя подменять это десятичным форматированием значений18/10/16 или
утверждать универсальную BCD-спецификацию без отдельного подтверждения.

`CManufactInfo` отдельно обнуляет три входа объекта версии, если:

- четыре bytes ASCII YYMM не являются цифрами;
- три bytes версии равны пробелам;
- YY меньше03 или не меньше90;
- YY=03 и MM меньше12.

YYMM находится в result9..12, то есть DATA6..9 по ранее установленному
отображению callback. Native-код здесь не проверяет календарные пределы
месяца: например,8999 проходит этот конкретный guard. UI может показать
байты даже при провале date guard; это другой путь, где также ограничивается
внутреннее поле dialog+8d. Нельзя объединять эти поведения в одно правило
«UI всегда скрывает старую версию».

Офлайн-анализатор теперь выдаёт `nativeVersionObservation`: исходные bytes,
строки отображения и результат guard объекта отдельно. `unitVersionKnown`,
`frameBindingVerified`, `cpuSelectionVerified`, `hardwareQualified` остаются
false: файл ответа сам по себе не доказывает адрес, SID или модель CPU.
Нулевой triplet сохраняется без объявления реальной версии `0.0.0`.
Никакие поля профиля компилятора/передачи автоматически не изменяются.

Craft finding `fnd_5`; статическая часть hyp_1 поддержана, но hypothesis
сохранена `testing`, поскольку её аппаратная часть ещё не выполнена.

## Отброшенная зацепка

В CX-P.exe найден прямой caller `12684c50 → 1285ad1e` импортированного
`IsExtendProtectionSupportedUnitVersion`. Его fallback аргумента версии
идёт через `GetDefaultVersion`, рядом находится `ProgChk` и
`GetProgramCheckFlagsFromOption`. Этот найденный путь относится к локальной
настройке Program Check. Он не измеряет версию или состояние защиты ПЛК;
все возможные indirect callers этим выводом не покрыты.

## Проверки и следующий аппаратный этап

`verify_cj1_manufacturing_consumer.py` воспроизводит переносимый
`testdata/cj1-manufacturing-consumer.json.gz`: закреплённые byte anchors,
menu entry, два блока RT_STRING, форматные строки, выбранные Ghidra exports
и четыре Craft disassembly records. С `--binary` дополнительно проверяет
полный SHA256 и50 диапазонов PE. Нормализация Services9 проверяется
отдельным `verify_cj1_manufacturing_framing.py`; новый verifier не заменяет его.

```text
python3 -B -m unittest discover -s tools/omron-research -p 'test_*.py'
python3 -B tools/omron-research/verify_cj1_manufacturing_consumer.py
python3 -B tools/omron-research/verify_cj1_manufacturing_consumer.py --binary extracted-iotablecs.ocx
python3 -B tools/omron-research/inspect_cj1_manufacturing_response.py retained-body.bin
```

Все41 Python-тест прошли. Новый PE replay:222 проверки,50 диапазонов.
Проверены повреждение selector/index/type/date/format/menu/label/source/scope,
неподтверждённый бинарник, границы YYMM, blank/zero triplet и различие
hex-отображения от десятичного. Go-код не менялся; Go-тесты не повторялись.
Количество проверок не является полнотой статического аудита.

Frozen evidence:
`C:/cx-proger/research/hardware/20261119-cj1-manufacturing-consumer-01`.
112 files excluding SHA256SUMS; manifest SHA256:
`9fbe1db124b8dd1e79cf7f2caa86d57d87dcc04a6a8cb4f8ec350021566cc4b4`.
Portable corpus SHA256:
`755f18eef830974f8ffa5386461ae601ff3983e18d81492ca87e79d8f41245b1`.
Предыдущие frozen-пакеты сохранены без изменений.

Далее в отдельном журнале read-only опыта:

1. На свежем соединении проверить0501/0601 и ожидаемую модель `.37`/`.36`.
2. Выполнить ограниченный0226 с selector0/index0, без retries/перебора,
   сохранив сырые request/response. При ненулевом end code остановить опыт.
3. Снять две копии manufacturing DATA и повторить статус. Проверить binding
   route/SID/command, длину, совпадение копий и raw bytes версии.
4. Сравнить версию с независимым источником: PLC Information/Unit
   Manufacturing Information либо маркировкой CPU. До этого не повышать
   аппаратную квалификацию профиля и не выбирать0307/0317 по новым bytes.
5. После наблюдения добавить bounded typed read API/retained hardware
   fixtures. Запись UM, финализация и восстановление остаются отдельными
   ещё не квалифицированными опытами.


Продолжение: [аппаратное CPU manufacturing read](omron-cj1-manufacturing-read-2026-11-19.md)
успешно выполнило описанные0226 на обоих CPU и проверило новый typed Go API.
Независимое подтверждение версии через PLC Information/маркировку ещё ожидается.
Исторические pending пункты выше относятся к статическому этапу.
