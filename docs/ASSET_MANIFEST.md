# Asset Manifest — Wishix v1.0

| Путь | Тип | Статус | Использование |
|---|---|---|---|
| `design/reference/original-desktop-mobile.png` | PNG | **утверждённый оригинальный визуальный эталон** | сравнение вёрстки |
| `design/reference/approved-brand-board.png` | PNG | **утверждённый брендборд** | цвета, компоненты, типографика |
| `design/reference/original-logo-crop.png` | PNG | точный фрагмент оригинального макета, **с фоном** | сравнение при перерисовке логотипа |
| `design/assets/logo-mark.svg` | SVG | векторная адаптация, **не точный исходник** | UI-прототип и дальнейшая доводка |
| `design/assets/logo-mark.png` | PNG 512 | рендер векторной адаптации, прозрачный фон | UI-прототип |
| `design/assets/logo-lockup.svg` | SVG | векторная адаптация + wordmark, **не точный исходник** | заголовок/экран входа |
| `design/assets/logo-lockup-light.svg` | SVG | светлый текст той же векторной адаптации | тёмный футер, копия во frontend |
| `design/assets/app-icon.svg` | SVG | векторная адаптация | источник под favicon/PWA |
| `design/assets/favicon.svg` | SVG | векторная адаптация знака Wishix | favicon приложения и демо |
| `design/assets/app-icon-512.png` | PNG 512 | рендер векторной адаптации | пример app icon |
| `design/assets/gift-hero.svg` | SVG | иллюстрация для примера, **не точный оригинал баннера** | hero demo, заменить после утверждения |
| `design/tokens.css` | CSS | **утверждённые цвета + практические UI-токены** | реализация |
| `design/tokens.json` | JSON | эквивалент токенов | интеграции/другие платформы |
| `../front/src/assets/fonts/InterVariable.woff2` | WOFF2 | локальный исходный Inter, SIL OFL 1.1 | интерфейс приложения; источник и hash в README рядом |
| `../front/src/assets/brand/google-g.png` | PNG | [официальный знак Google](https://developers.google.com/identity/branding-guidelines) | кнопка входа и подключения Google |
| `../front/src/assets/fonts/GoogleSans-*.woff2` | WOFF2 | Google Sans Medium, SIL OFL 1.1 | только кнопка Google; источник и лицензия рядом |
| `design/preview/my-390.png`, `my-1440.png` | PNG | контрольные скриншоты `/my`, 04.11.2026 | визуальная приёмка; аккаунт подставлен только в проверочном браузере |
| `design/preview/wishes-390.png`, `wishes-1440.png` | PNG | текущая общая коллекция, 04.11.2026 | реальные записи только из изолированной тестовой БД |
| `design/preview/new-wish-390.png`, `new-wish-1440.png` | PNG | отдельная форма добавления, 04.11.2026 | визуальная приёмка EN/RU/RO и адаптивности |
| `design/preview/wish-images-390.png`, `wish-images-1440.png` | PNG | коллекция с загруженным изображением, 04.11.2026 | тестовые запись и файл только из изолированной среды |
| `design/preview/wish-image-form-390.png`, `wish-image-form-1440.png` | PNG | выбор файла и предпросмотр, 04.11.2026 | визуальная приёмка EN/RU/RO и адаптивности |
| `design/preview/wish-form-links-first-390.png`, `wish-form-links-first-1440.png` | PNG | актуальная форма: ссылки первым блоком, 04.11.2026 | адаптивность, порядок клавиатурной навигации и ручное добавление без ссылки |

## Что осталось для production

1. При необходимости сделать максимально точный векторный master утверждённого знака и экспорт в SVG/PNG. Текущий вектор — аккуратная самостоятельная адаптация.
2. Утвердить отдельную иллюстрацию подарка в hero (если нужно пиксельное совпадение с исходным макетом).
3. Подготовить реальные, законно используемые фото/обложки или безопасные placeholders.
4. Проверить и сформировать platform-specific иконки для PWA, Android/iOS при появлении таких платформ. Наличие `app-icon-512.png` не означает готовности полного набора store-иконок.
5. В приложении Inter раздаётся локально с OFL-лицензией из `front/src/assets/fonts/`. Статическое демо продолжает использовать свой способ подключения шрифта.
