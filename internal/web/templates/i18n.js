// i18n.js - Lightweight internationalization system for ExpenseOwl

class I18n {
    constructor() {
        this.currentLocale = this.getStoredLocale() || this.detectLocale();
        this.translations = {};
        this.defaultLocale = 'en';
        this.availableLocales = ['en', 'ms', 'es', 'fr', 'de']; // Extend as needed
    }

    // Detect browser locale
    detectLocale() {
        const browserLocale = navigator.language || navigator.userLanguage;
        const langCode = browserLocale.split('-')[0]; // e.g., 'en-US' -> 'en'
        return this.availableLocales.includes(langCode) ? langCode : this.defaultLocale;
    }

    // Get stored locale from localStorage
    getStoredLocale() {
        return localStorage.getItem('locale');
    }

    // Set and store locale
    async setLocale(locale) {
        if (!this.availableLocales.includes(locale)) {
            console.warn(`Locale ${locale} not available, falling back to ${this.defaultLocale}`);
            locale = this.defaultLocale;
        }
        this.currentLocale = locale;
        localStorage.setItem('locale', locale);
        await this.loadTranslations(locale);
        this.updatePageTranslations();
        this.updateHtmlLang();
        // Dispatch event for components to re-render with new locale
        window.dispatchEvent(new CustomEvent('localeChanged', { detail: { locale } }));
    }

    // Load translation file
    async loadTranslations(locale) {
        try {
            const response = await fetch(`/locales/${locale}.json`);
            if (!response.ok) throw new Error(`Failed to load locale: ${locale}`);
            this.translations = await response.json();
        } catch (error) {
            console.error('Translation load error:', error);
            if (locale !== this.defaultLocale) {
                console.log('Falling back to default locale');
                const response = await fetch(`/locales/${this.defaultLocale}.json`);
                this.translations = await response.json();
            }
        }
    }

    // Get translation by key (supports nested keys like "dashboard.title")
    t(key, interpolations = {}) {
        const keys = key.split('.');
        let translation = this.translations;

        for (const k of keys) {
            if (translation && typeof translation === 'object' && k in translation) {
                translation = translation[k];
            } else {
                console.warn(`Translation key not found: ${key}`);
                return key; // Return key if translation not found
            }
        }

        // Handle interpolations: t('tags_create', { tag: 'work' })
        if (typeof translation === 'string' && Object.keys(interpolations).length > 0) {
            return translation.replace(/\{(\w+)\}/g, (match, key) => {
                return interpolations[key] !== undefined ? interpolations[key] : match;
            });
        }

        return translation;
    }

    // Update all elements with data-i18n attribute
    updatePageTranslations() {
        document.querySelectorAll('[data-i18n]').forEach(element => {
            const key = element.getAttribute('data-i18n');
            const translation = this.t(key);

            // Handle different element types
            if (element.tagName === 'INPUT' && (element.type === 'text' || element.type === 'number')) {
                element.placeholder = translation;
            } else if (element.hasAttribute('data-i18n-attr')) {
                // For custom attributes like data-tooltip
                const attr = element.getAttribute('data-i18n-attr');
                element.setAttribute(attr, translation);
            } else {
                element.textContent = translation;
            }
        });
    }

    // Update HTML lang attribute
    updateHtmlLang() {
        document.documentElement.setAttribute('lang', this.currentLocale);
    }

    // Get formatted month name
    formatMonth(date) {
        const monthIndex = date.getMonth();
        const monthKeys = [
            'months.january', 'months.february', 'months.march', 'months.april',
            'months.may', 'months.june', 'months.july', 'months.august',
            'months.september', 'months.october', 'months.november', 'months.december'
        ];
        const monthName = this.t(monthKeys[monthIndex]);
        return `${monthName} ${date.getFullYear()}`;
    }

    // Get locale-aware date formatter
    getDateFormat() {
        return new Intl.DateTimeFormat(this.currentLocale, {
            month: 'short',
            day: 'numeric',
            year: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
            timeZoneName: 'short'
        });
    }

    // Get available locales for selector
    getAvailableLocales() {
        return this.availableLocales;
    }

    // Initialize on page load
    async init() {
        await this.loadTranslations(this.currentLocale);
        this.updatePageTranslations();
        this.updateHtmlLang();
    }
}

// Create global instance
const i18n = new I18n();
