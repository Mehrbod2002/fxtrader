package constants

var TradeRetcodes = map[int]map[string]string{
	10004: {
		"en": "Requote",
		"fa": "قیمت جدید ارائه شد",
	},
	10006: {
		"en": "Request rejected",
		"fa": "درخواست رد شد",
	},
	10007: {
		"en": "Request canceled by trader",
		"fa": "درخواست توسط معامله‌گر لغو شد",
	},
	10008: {
		"en": "Order placed",
		"fa": "سفارش ثبت شد",
	},
	10009: {
		"en": "Request completed",
		"fa": "درخواست تکمیل شد",
	},
	10010: {
		"en": "Only part of the request was completed",
		"fa": "تنها بخشی از درخواست تکمیل شد",
	},
	10011: {
		"en": "Request processing error",
		"fa": "خطا در پردازش درخواست",
	},
	10012: {
		"en": "Request canceled by timeout",
		"fa": "درخواست به دلیل اتمام زمان لغو شد",
	},
	10013: {
		"en": "Invalid request",
		"fa": "درخواست نامعتبر",
	},
	10014: {
		"en": "Invalid volume in the request",
		"fa": "حجم نامعتبر در درخواست",
	},
	10015: {
		"en": "Invalid price in the request",
		"fa": "قیمت نامعتبر در درخواست",
	},
	10016: {
		"en": "Invalid stops in the request",
		"fa": "حد ضرر یا سود نامعتبر در درخواست",
	},
	10017: {
		"en": "Trade is disabled",
		"fa": "معامله غیرفعال است",
	},
	10018: {
		"en": "Market is closed",
		"fa": "بازار بسته است",
	},
	10019: {
		"en": "There is not enough money to complete the request",
		"fa": "موجودی کافی برای تکمیل درخواست وجود ندارد",
	},
	10020: {
		"en": "Prices changed",
		"fa": "قیمت‌ها تغییر کردند",
	},
	10021: {
		"en": "There are no quotes to process the request",
		"fa": "هیچ قیمتی برای پردازش درخواست وجود ندارد",
	},
	10022: {
		"en": "Invalid order expiration date in the request",
		"fa": "تاریخ انقضای سفارش نامعتبر است",
	},
	10023: {
		"en": "Order state changed",
		"fa": "وضعیت سفارش تغییر کرد",
	},
	10024: {
		"en": "Too frequent requests",
		"fa": "درخواست‌های بیش از حد",
	},
	10025: {
		"en": "No changes in request",
		"fa": "تغییری در درخواست وجود ندارد",
	},
	10026: {
		"en": "Autotrading disabled by server",
		"fa": "معاملات خودکار توسط سرور غیرفعال شد",
	},
	10027: {
		"en": "Autotrading disabled by client terminal",
		"fa": "معاملات خودکار توسط ترمینال کاربر غیرفعال شد",
	},
	10028: {
		"en": "Request locked for processing",
		"fa": "درخواست برای پردازش قفل شده است",
	},
	10029: {
		"en": "Order or position frozen",
		"fa": "سفارش یا موقعیت مسدود شده است",
	},
	10030: {
		"en": "Invalid order filling type",
		"fa": "نوع تکمیل سفارش نامعتبر است",
	},
	10031: {
		"en": "No connection with the trade server",
		"fa": "اتصال به سرور معاملاتی وجود ندارد",
	},
	10032: {
		"en": "Operation is allowed only for live accounts",
		"fa": "عملیات تنها برای حساب‌های واقعی مجاز است",
	},
	10033: {
		"en": "The number of pending orders has reached the limit",
		"fa": "تعداد سفارش‌های معلق به حد مجاز رسیده است",
	},
	10034: {
		"en": "The volume of orders and positions for the symbol has reached the limit",
		"fa": "حجم سفارش‌ها و موقعیت‌ها برای نماد به حد مجاز رسیده است",
	},
	10035: {
		"en": "Incorrect or prohibited order type",
		"fa": "نوع سفارش نادرست یا ممنوع است",
	},
	10036: {
		"en": "Position with the specified identifier has already been closed",
		"fa": "موقعیت با شناسه مشخص شده قبلاً بسته شده است",
	},
	10038: {
		"en": "A close volume exceeds the current position volume",
		"fa": "حجم بسته شدن بیشتر از حجم موقعیت فعلی است",
	},
	10039: {
		"en": "A close order already exists for this position",
		"fa": "یک سفارش بستن برای این موقعیت از قبل وجود دارد",
	},
	10040: {
		"en": "The number of open positions has reached the limit",
		"fa": "تعداد موقعیت‌های باز به حد مجاز رسیده است",
	},
	10041: {
		"en": "The pending order activation request is rejected, the order is canceled",
		"fa": "درخواست فعال‌سازی سفارش معلق رد شد، سفارش لغو شد",
	},
	10042: {
		"en": "Only long positions are allowed",
		"fa": "تنها موقعیت‌های خرید مجاز هستند",
	},
	10043: {
		"en": "Only short positions are allowed",
		"fa": "تنها موقعیت‌های فروش مجاز هستند",
	},
	10044: {
		"en": "Only position closing is allowed",
		"fa": "تنها بستن موقعیت‌ها مجاز است",
	},
	10045: {
		"en": "Position closing is allowed only by FIFO rule",
		"fa": "بستن موقعیت تنها طبق قانون FIFO مجاز است",
	},
	10046: {
		"en": "Opposite positions on a single symbol are disabled",
		"fa": "موقعیت‌های مخالف روی یک نماد غیرفعال هستند",
	},
}
