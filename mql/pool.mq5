#property copyright "Your Name"
#property link      "https://yourwebsite.com"
#property version   "2.01"

#include <Trade\Trade.mqh>
#include <Frame.mqh>
#include <Socket.mqh>
#include <WebsocketClient.mqh>
#include <Lib\Json.mqh>      // CJAVal-based Json.mqh

// Input parameters
input string InpWebSocketURL = "127.0.0.1";              // WebSocket server URL
input int InputPort = 7003;                              // WebSocket server port
input string InpClientID = "MT5_Client_1";               // Unique client ID
input int InpPingInterval = 30;                          // Ping interval (seconds)
input int InpReconnectDelay = 2;                         // Initial reconnect delay (seconds)
input int InpMaxReconnectAttempts = 5;                   // Max reconnect attempts
input double InpSpreadTolerance = 0.0001;                // Spread tolerance for matching
input int InpTimerInterval = 500;                        // Timer interval (ms)

// Trade pool structure
struct PoolTrade {
   string trade_id;
   string user_id;
   string symbol;
   string trade_type;
   string order_type;
   string account_type; // "DEMO" or "REAL"
   int leverage;
   double volume;
   double entry_price;
   double stop_loss;
   double take_profit;
   datetime timestamp;
   datetime expiration;
   ulong ticket;
};

// Trade pool manager class
class CTradePoolManager {
public:
   PoolTrade pool[];
   int pool_size;
   CTrade trade;
   string message_queue[];

   bool ValidateTrade(PoolTrade &t) {
      if (t.trade_type != "BUY" && t.trade_type != "SELL") return false;
      if (t.order_type != "MARKET" && t.order_type != "BUY_LIMIT" && t.order_type != "SELL_LIMIT" &&
          t.order_type != "BUY_STOP" && t.order_type != "SELL_STOP") return false;
      if (t.account_type != "DEMO" && t.account_type != "REAL") return false;
      if (t.leverage <= 0 || t.volume <= 0) return false;
      if (t.order_type != "MARKET" && t.entry_price <= 0) return false;
      if (t.stop_loss < 0 || t.take_profit < 0) return false;
      if (t.expiration > 0 && t.expiration <= TimeCurrent()) return false;
      return true;
   }

   int FindMatchingTrade(PoolTrade &new_trade) {
      for (int i = 0; i < pool_size; i++) {
         if (pool[i].trade_id == "" || pool[i].ticket == 0) continue;
         if (CanMatchTrades(new_trade, pool[i])) return i;
      }
      return -1;
   }

   bool CanMatchTrades(PoolTrade &trade1, PoolTrade &trade2) {
      if (trade1.trade_type == trade2.trade_type || trade1.symbol != trade2.symbol ||
          trade1.volume != trade2.volume || trade1.account_type != trade2.account_type) return false;
      if (trade1.order_type == "MARKET" || trade2.order_type == "MARKET") return true;
      if ((trade1.order_type == "BUY_LIMIT" || trade1.order_type == "SELL_LIMIT") &&
          (trade2.order_type == "BUY_LIMIT" || trade2.order_type == "SELL_LIMIT")) {
         return MathAbs(trade1.entry_price - trade2.entry_price) <= InpSpreadTolerance;
      }
      return false;
   }

public:
   CTradePoolManager() : pool_size(0) {}

   void AddToPool(PoolTrade &t) {
      ArrayResize(pool, pool_size + 1);
      pool[pool_size] = t;
      pool_size++;
      Print("Added trade to pool: ", t.trade_id, ", Symbol: ", t.symbol, ", Type: ", t.trade_type, ", Order: ", t.order_type);
   }

   void CleanupTradePool() {
      for (int i = pool_size - 1; i >= 0; i--) {
         if (pool[i].trade_id == "") {
            ArrayRemove(pool, i, 1);
            pool_size--;
         }
      }
   }

   bool ExecuteMarketTrade(PoolTrade &t) {
      double price = t.order_type == "MARKET" ? SymbolInfoDouble(t.symbol, t.trade_type == "BUY" ? SYMBOL_ASK : SYMBOL_BID) : t.entry_price;
      string comment = "Trade";
      bool success = false;
      ulong ticket = 0;

      trade.SetExpertMagicNumber(StringToInteger(t.trade_id));
      if (t.trade_type == "BUY") {
         success = trade.Buy(t.volume, t.symbol, price, t.stop_loss, t.take_profit, comment);
         ticket = trade.ResultOrder();
      } else {
         success = trade.Sell(t.volume, t.symbol, price, t.stop_loss, t.take_profit, comment);
         ticket = trade.ResultOrder();
      }

      if (success) {
         t.ticket = ticket;
         Print("Executed trade: ", t.trade_id, " Symbol: ", t.symbol, " Type: ", t.trade_type, " at price: ", price);
      } else {
         t.trade_id = "";
         Print("Failed to execute trade: ", t.trade_id, ". Error: ", trade.ResultComment());
      }
      return success;
   }

   bool PlacePendingOrder(PoolTrade &t) {
      string comment = "Pending";
      bool success = false;
      t.ticket = 0;

      trade.SetExpertMagicNumber(StringToInteger(t.trade_id));
      if (t.order_type == "BUY_LIMIT") {
         success = trade.BuyLimit(t.volume, t.entry_price, t.symbol, t.stop_loss, t.take_profit, ORDER_TIME_SPECIFIED, t.expiration, comment);
      } else if (t.order_type == "SELL_LIMIT") {
         success = trade.SellLimit(t.volume, t.entry_price, t.symbol, t.stop_loss, t.take_profit, ORDER_TIME_SPECIFIED, t.expiration, comment);
      } else if (t.order_type == "BUY_STOP") {
         success = trade.BuyStop(t.volume, t.entry_price, t.symbol, t.stop_loss, t.take_profit, ORDER_TIME_SPECIFIED, t.expiration, comment);
      } else if (t.order_type == "SELL_STOP") {
         success = trade.SellStop(t.volume, t.entry_price, t.symbol, t.stop_loss, t.take_profit, ORDER_TIME_SPECIFIED, t.expiration, comment);
      }

      if (success) {
         t.ticket = trade.ResultOrder();
         Print("Placed pending order: ", t.trade_id, " Symbol: ", t.symbol, " Type: ", t.trade_type, " Order: ", t.order_type);
      } else {
         Print("Failed to place pending order: ", t.trade_id, ". Error: ", trade.ResultComment());
      }
      return success;
   }

   void ExecuteMatchedTrades(PoolTrade &trade1, PoolTrade &trade2) {
      double match_price = (trade1.order_type == "MARKET" || trade2.order_type == "MARKET")
                          ? SymbolInfoDouble(trade1.symbol, SYMBOL_BID)
                          : (trade1.entry_price + trade2.entry_price) / 2.0;
      string comment = "TradeMatch";
      bool success = true;
      ulong ticket1 = 0, ticket2 = 0;

      trade.SetExpertMagicNumber(StringToInteger(trade1.trade_id));
      if (trade1.trade_type == "BUY") {
         success &= trade.Buy(trade1.volume, trade1.symbol, match_price, trade1.stop_loss, trade1.take_profit, comment);
         ticket1 = trade.ResultOrder();
         trade.SetExpertMagicNumber(StringToInteger(trade2.trade_id));
         success &= trade.Sell(trade2.volume, trade2.symbol, match_price, trade2.stop_loss, trade2.take_profit, comment);
         ticket2 = trade.ResultOrder();
      } else {
         success &= trade.Buy(trade2.volume, trade2.symbol, match_price, trade2.stop_loss, trade2.take_profit, comment);
         ticket2 = trade.ResultOrder();
         trade.SetExpertMagicNumber(StringToInteger(trade1.trade_id));
         success &= trade.Sell(trade1.volume, trade1.symbol, match_price, trade1.stop_loss, trade1.take_profit, comment);
         ticket1 = trade.ResultOrder();
      }

      string status = success ? "EXECUTED" : "FAILED";
      SendTradeResponse(trade1.trade_id, trade1.user_id, trade1.account_type, status, trade2.trade_id, ticket1, success ? "" : trade.ResultComment());
      SendTradeResponse(trade2.trade_id, trade2.user_id, trade2.account_type, status, trade1.trade_id, ticket2, success ? "" : trade.ResultComment());

      if (success) {
         for (int i = 0; i < pool_size; i++) {
            if (pool[i].trade_id == trade2.trade_id) {
               if (pool[i].ticket > 0) trade.OrderDelete(pool[i].ticket);
               pool[i].trade_id = "";
               break;
            }
         }
         Print("Matched trades: ", trade1.trade_id, " and ", trade2.trade_id, " at price: ", match_price);
      } else {
         Print("Failed to match trades: ", trade1.trade_id, " and ", trade2.trade_id, ". Error: ", trade.ResultComment());
      }
   }

   bool HandleTradeRequest(CJAVal &json) {
      string symbol = json["symbol"].ToStr();
      if (!SymbolSelect(symbol, true)) {
         SendTradeResponse(json["trade_id"].ToStr(), json["user_id"].ToStr(), json["account_type"].ToStr(), "FAILED", "", 0, "Invalid symbol");
         return false;
      }

      double volume = json["volume"].ToDbl();
      double min_volume = SymbolInfoDouble(symbol, SYMBOL_VOLUME_MIN);
      double max_volume = SymbolInfoDouble(symbol, SYMBOL_VOLUME_MAX);
      if (volume < min_volume || volume > max_volume) {
         SendTradeResponse(json["trade_id"].ToStr(), json["user_id"].ToStr(), json["account_type"].ToStr(), "FAILED", "", 0, "Invalid volume");
         return false;
      }

      PoolTrade new_trade;
      new_trade.trade_id = json["trade_id"].ToStr();
      new_trade.user_id = json["user_id"].ToStr();
      new_trade.symbol = symbol;
      new_trade.trade_type = json["trade_type"].ToStr();
      new_trade.order_type = json["order_type"].ToStr();
      new_trade.account_type = json["account_type"].ToStr();
      new_trade.leverage = (int)json["leverage"].ToInt();
      new_trade.volume = volume;
      new_trade.entry_price = json["entry_price"].ToDbl();
      new_trade.stop_loss = json["stop_loss"].ToDbl();
      new_trade.take_profit = json["take_profit"].ToDbl();
      new_trade.timestamp = (datetime)json["timestamp"].ToInt();
      new_trade.expiration = json["expiration"].ToInt() > 0 ? (datetime)json["expiration"].ToInt() : 0;
      new_trade.ticket = 0;

      if (!ValidateTrade(new_trade)) {
         SendTradeResponse(new_trade.trade_id, new_trade.user_id, new_trade.account_type, "FAILED", "", 0, "Invalid trade parameters");
         return false;
      }

      int match_index = FindMatchingTrade(new_trade);
      if (match_index >= 0) {
         ExecuteMatchedTrades(new_trade, pool[match_index]);
         if (new_trade.trade_id != "") {
            AddToPool(new_trade);
            SendTradeResponse(new_trade.trade_id, new_trade.user_id, new_trade.account_type, "PENDING", "", 0, "");
         }
      } else {
         if (new_trade.order_type == "MARKET") {
            if (ExecuteMarketTrade(new_trade) && new_trade.trade_id != "") {
               AddToPool(new_trade);
               SendTradeResponse(new_trade.trade_id, new_trade.user_id, new_trade.account_type, "PENDING", "", 0, "");
            }
         } else {
            bool success = PlacePendingOrder(new_trade);
            if (success && new_trade.ticket > 0) {
               AddToPool(new_trade);
               SendTradeResponse(new_trade.trade_id, new_trade.user_id, new_trade.account_type, "PENDING", "", new_trade.ticket, "");
            } else {
               SendTradeResponse(new_trade.trade_id, new_trade.user_id, new_trade.account_type, "FAILED", "", 0, "Failed to place pending order");
            }
         }
      }
      return true;
   }

   void ProcessTick() {
      for (int i = 0; i < pool_size; i++) {
         if (pool[i].trade_id == "" || pool[i].ticket == 0) continue;
         if (pool[i].order_type == "BUY_STOP" || pool[i].order_type == "SELL_STOP") {
            double market_price = SymbolInfoDouble(pool[i].symbol, SYMBOL_BID);
            bool triggered = (pool[i].order_type == "BUY_STOP" && market_price >= pool[i].entry_price) ||
                            (pool[i].order_type == "SELL_STOP" && market_price <= pool[i].entry_price);
            if (triggered) ExecuteMarketTrade(pool[i]);
         }
      }
   }

   bool SendTradeResponse(string trade_id, string user_id, string account_type, string status, string matched_trade_id, ulong ticket, string error) {
      CJAVal response_json;
      response_json["type"] = "trade_response";
      response_json["trade_id"] = trade_id;
      response_json["user_id"] = user_id;
      response_json["account_type"] = account_type;
      response_json["status"] = status;
      response_json["matched_trade_id"] = matched_trade_id;
      response_json["ticket"] = (long)ticket;
      response_json["timestamp"] = (long)TimeCurrent();
      if (error != "") response_json["error"] = error;

      string json_str = response_json.Serialize();
      if (json_str == "") {
         Print("Failed to serialize trade response");
         return false;
      }

      if (ws.SendString(json_str)) {
         Print("Sent trade response: ", json_str);
         return true;
      } else {
         Print("Failed to send trade response: ", json_str);
         QueueMessage(json_str);
         return false;
      }
   }

   bool SendBalanceResponse(string user_id, string account_type, double balance, string error) {
      CJAVal response_json;
      response_json["type"] = "balance_response";
      response_json["user_id"] = user_id;
      response_json["account_type"] = account_type;
      response_json["balance"] = balance;
      response_json["timestamp"] = (long)TimeCurrent();
      if (error != "") response_json["error"] = error;

      string json_str = response_json.Serialize();
      if (json_str == "") {
         Print("Failed to serialize balance response");
         return false;
      }

      if (ws.SendString(json_str)) {
         Print("Sent balance response: ", json_str);
         return true;
      } else {
         Print("Failed to send balance response: ", json_str);
         QueueMessage(json_str);
         return false;
      }
   }

   void CloseOrder(string user_id, string trade_id, string account_type, ulong ticket) {
      bool success = false;
      string error = "";

      if (PositionSelectByTicket(ticket)) {
         string comment = PositionGetString(POSITION_COMMENT);
         if (comment == "Trade" && IsUserTrade(user_id, trade_id, account_type)) {
            success = trade.PositionClose(ticket);
            error = success ? "" : trade.ResultComment();
         } else {
            error = "Ticket does not belong to user or trade";
         }
      } else if (OrderSelect(ticket)) {
         string comment = OrderGetString(ORDER_COMMENT);
         if (comment == "Pending" && IsUserTrade(user_id, trade_id, account_type)) {
            success = trade.OrderDelete(ticket);
            error = success ? "" : trade.ResultComment();
            for (int i = 0; i < pool_size; i++) {
               if (pool[i].ticket == ticket && pool[i].trade_id == trade_id) {
                  pool[i].trade_id = "";
                  break;
               }
            }
         } else {
            error = "Ticket does not belong to user or trade";
         }
      } else {
         error = "Invalid ticket";
      }

      string status = success ? "CLOSED" : "FAILED";
      SendTradeResponse(trade_id, user_id, account_type, status, "", ticket, error);
      if (success) {
         Print("Closed order: Ticket ", ticket, " for trade_id: ", trade_id);
      } else {
         Print("Failed to close order: Ticket ", ticket, " for trade_id: ", trade_id, ". Error: ", error);
      }
   }

   void CloseProfitOrders(string user_id, string trade_id, string account_type) {
      bool success = true;
      string error = "";
      CJAVal closed_tickets(NULL, jtARRAY);

      for (int i = PositionsTotal() - 1; i >= 0; i--) {
         ulong ticket = PositionGetTicket(i);
         if (PositionSelectByTicket(ticket)) {
            string comment = PositionGetString(POSITION_COMMENT);
            if (comment == "Trade" && IsUserTrade(user_id, trade_id, account_type)) {
               double profit = PositionGetDouble(POSITION_PROFIT);
               if (profit > 0) {
                  if (trade.PositionClose(ticket)) {
                     CJAVal ticket_obj;
                     ticket_obj = (long)ticket;
                     closed_tickets.Add(ticket_obj);
                  } else {
                     success = false;
                     error = trade.ResultComment();
                  }
               }
            }
         }
      }

      string status = success ? "CLOSED" : "FAILED";
      CJAVal response_json;
      response_json["type"] = "trade_response";
      response_json["trade_id"] = trade_id;
      response_json["user_id"] = user_id;
      response_json["account_type"] = account_type;
      response_json["status"] = status;
      response_json["closed_tickets"] = closed_tickets;
      response_json["timestamp"] = (long)TimeCurrent();
      if (error != "") response_json["error"] = error;

      string json_str = response_json.Serialize();
      if (json_str != "" && ws.SendString(json_str)) {
         Print("Sent profit orders close response: ", json_str);
      } else {
         Print("Failed to send profit orders close response: ", json_str);
         QueueMessage(json_str);
      }
   }

   void CloseLossOrders(string user_id, string trade_id, string account_type) {
      bool success = true;
      string error = "";
      CJAVal closed_tickets(NULL, jtARRAY);

      for (int i = PositionsTotal() - 1; i >= 0; i--) {
         ulong ticket = PositionGetTicket(i);
         if (PositionSelectByTicket(ticket)) {
            string comment = PositionGetString(POSITION_COMMENT);
            if (comment == "Trade" && IsUserTrade(user_id, trade_id, account_type)) {
               double profit = PositionGetDouble(POSITION_PROFIT);
               if (profit < 0) {
                  if (trade.PositionClose(ticket)) {
                     CJAVal ticket_obj;
                     ticket_obj = (long)ticket;
                     closed_tickets.Add(ticket_obj);
                  } else {
                     success = false;
                     error = trade.ResultComment();
                  }
               }
            }
         }
      }

      string status = success ? "CLOSED" : "FAILED";
      CJAVal response_json;
      response_json["type"] = "trade_response";
      response_json["trade_id"] = trade_id;
      response_json["user_id"] = user_id;
      response_json["account_type"] = account_type;
      response_json["status"] = status;
      response_json["closed_tickets"] = closed_tickets;
      response_json["timestamp"] = (long)TimeCurrent();
      if (error != "") response_json["error"] = error;

      string json_str = response_json.Serialize();
      if (json_str != "" && ws.SendString(json_str)) {
         Print("Sent loss orders close response: ", json_str);
      } else {
         Print("Failed to send loss orders close response: ", json_str);
         QueueMessage(json_str);
      }
   }

   void StreamOpenOrders(string user_id, string trade_id, string account_type) {
      CJAVal open_orders(NULL, jtARRAY);

      for (int i = 0; i < PositionsTotal(); i++) {
         ulong ticket = PositionGetTicket(i);
         if (PositionSelectByTicket(ticket)) {
            string comment = PositionGetString(POSITION_COMMENT);
            if (comment == "Trade" && IsUserTrade(user_id, trade_id, account_type)) {
               CJAVal order(NULL, jtOBJ);
               order["ticket"] = (long)ticket;
               order["symbol"] = PositionGetString(POSITION_SYMBOL);
               order["type"] = PositionGetInteger(POSITION_TYPE) == POSITION_TYPE_BUY ? "BUY" : "SELL";
               order["volume"] = PositionGetDouble(POSITION_VOLUME);
               order["entry_price"] = PositionGetDouble(POSITION_PRICE_OPEN);
               order["stop_loss"] = PositionGetDouble(POSITION_SL);
               order["take_profit"] = PositionGetDouble(POSITION_TP);
               order["profit"] = PositionGetDouble(POSITION_PROFIT);
               order["timestamp"] = (long)PositionGetInteger(POSITION_TIME);
               open_orders.Add(order);
            }
         }
      }

      for (int i = 0; i < OrdersTotal(); i++) {
         ulong ticket = OrderGetTicket(i);
         if (OrderSelect(ticket)) {
            string comment = OrderGetString(ORDER_COMMENT);
            if (comment == "Pending" && IsUserTrade(user_id, trade_id, account_type)) {
               CJAVal order(NULL, jtOBJ);
               order["ticket"] = (long)ticket;
               order["symbol"] = OrderGetString(ORDER_SYMBOL);
               order["type"] = OrderGetInteger(ORDER_TYPE) == ORDER_TYPE_BUY_LIMIT ? "BUY_LIMIT" :
                              OrderGetInteger(ORDER_TYPE) == ORDER_TYPE_SELL_LIMIT ? "SELL_LIMIT" :
                              OrderGetInteger(ORDER_TYPE) == ORDER_TYPE_BUY_STOP ? "BUY_STOP" : "SELL_STOP";
               order["volume"] = OrderGetDouble(ORDER_VOLUME_CURRENT);
               order["entry_price"] = OrderGetDouble(ORDER_PRICE_OPEN);
               order["stop_loss"] = OrderGetDouble(ORDER_SL);
               order["take_profit"] = OrderGetDouble(ORDER_TP);
               order["timestamp"] = (long)OrderGetInteger(ORDER_TIME_SETUP);
               open_orders.Add(order);
            }
         }
      }

      CJAVal response_json;
      response_json["type"] = "open_orders_response";
      response_json["trade_id"] = trade_id;
      response_json["user_id"] = user_id;
      response_json["account_type"] = account_type;
      response_json["orders"] = open_orders;
      response_json["timestamp"] = (long)TimeCurrent();

      string json_str = response_json.Serialize();
      if (json_str != "" && ws.SendString(json_str)) {
         Print("Sent open orders stream: ", json_str);
      } else {
         Print("Failed to send open orders stream: ", json_str);
         QueueMessage(json_str);
      }
   }

   bool IsUserTrade(string user_id, string trade_id, string account_type) {
      for (int i = 0; i < pool_size; i++) {
         if (pool[i].trade_id == trade_id && pool[i].user_id == user_id && pool[i].account_type == account_type) {
            return true;
         }
      }
      return false;
   }

   void QueueMessage(string message) {
      ArrayResize(message_queue, ArraySize(message_queue) + 1);
      message_queue[ArraySize(message_queue) - 1] = message;
      Print("Queued message: ", message);
   }
};

class CWebSocketManager {
private:
   CWebSocketClient ws; // WebSocket client instance
   datetime last_ping_sent;
   datetime last_reconnect_attempt;
   int reconnect_attempts;

   bool SendMessage(const string message) {
      if (ws.SendString(message)) {
         Print("Sent message: ", message);
         return true;
      } else {
         Print("Failed to send message: ", message);
         return false;
      }
   }

public:
   CWebSocketManager() : last_ping_sent(0), last_reconnect_attempt(0), reconnect_attempts(0) {}

   bool Initialize(const string url, const int port) {
      if (!ws.Connect(url, port, 10000)) {
         Print("Failed to connect to WebSocket server: ", url);
         for (int i = 0; i < InpMaxReconnectAttempts; i++) {
            Sleep(InpReconnectDelay * 1000);
            if (ws.Connect(url, port, 10000)) {
               if (SendHandshake()) {
                  Print("Connected to WebSocket server: ", url);
                  return true;
               }
            }
            if (i == InpMaxReconnectAttempts - 1) {
               Alert("Failed to initialize WebSocket after ", InpMaxReconnectAttempts, " attempts");
               return false;
            }
         }
      }
      return true;
   }

   bool SendHandshake() {
      CJAVal json;
      json["type"] = "handshake";
      json["client_id"] = InpClientID;
      json["timestamp"] = (long)TimeCurrent();
      string json_str = json.Serialize();
      if (json_str == "") {
         Print("Failed to serialize handshake");
         return false;
      }
      return SendMessage(json_str);
   }

   void ProcessMessages(CTradePoolManager &pool_managers) {
      // First, check if there is data to read
      int rx_size = ws.Readable();
      if (rx_size <= 0) {
         return; // No data to process
      }

      // Read frames from the WebSocket
      CFrame out[];
      uint total_len = ws.Read(out);
      if (total_len == 0) {
         Print("No valid frames received or receive buffer empty");
         return;
      }

      // Process each frame
      for (int i = 0; i < ArraySize(out); i++) {
         if (out[i].MessageType() == TEXT_FRAME) {
            string message = out[i].ToString();
            if (message == NULL) {
               Print("Failed to convert frame to string");
               continue;
            }
            Print("Received: ", message);

            CJAVal json;
            if (json.Deserialize(message)) {
               string msg_type = json["type"].ToStr();
               if (msg_type == "") {
                  Print("Missing 'type' in JSON");
                  continue;
               }

               if (msg_type == "handshake_response") {
                  reconnect_attempts = 0;
                  Print("Received handshake response");
               } else if (msg_type == "trade_request") {
                  pool_managers.HandleTradeRequest(json);
               } else if (msg_type == "balance_request") {
                  string user_id = json["user_id"].ToStr();
                  string account_type = json["account_type"].ToStr();
                  double balance = AccountInfoDouble(ACCOUNT_BALANCE);
                  pool_managers.SendBalanceResponse(user_id, account_type, balance, "");
               } else if (msg_type == "close_order") {
                  string user_id = json["user_id"].ToStr();
                  string trade_id = json["trade_id"].ToStr();
                  string account_type = json["account_type"].ToStr();
                  ulong ticket = (ulong)json["ticket"].ToInt();
                  pool_managers.CloseOrder(user_id, trade_id, account_type, ticket);
               } else if (msg_type == "close_profit_orders") {
                  string user_id = json["user_id"].ToStr();
                  string trade_id = json["trade_id"].ToStr();
                  string account_type = json["account_type"].ToStr();
                  pool_managers.CloseProfitOrders(user_id, trade_id, account_type);
               } else if (msg_type == "close_loss_orders") {
                  string user_id = json["user_id"].ToStr();
                  string trade_id = json["trade_id"].ToStr();
                  string account_type = json["account_type"].ToStr();
                  pool_managers.CloseLossOrders(user_id, trade_id, account_type);
               } else if (msg_type == "stream_open_orders") {
                  string user_id = json["user_id"].ToStr();
                  string trade_id = json["trade_id"].ToStr();
                  string account_type = json["account_type"].ToStr();
                  pool_managers.StreamOpenOrders(user_id, trade_id, account_type);
               } else if (msg_type == "ping") {
                  CJAVal pong_json;
                  pong_json["type"] = "pong";
                  pong_json["timestamp"] = (long)TimeCurrent();
                  string pong_str = pong_json.Serialize();
                  if (pong_str != "" && SendMessage(pong_str)) {
                     Print("Sent pong: ", pong_str);
                  } else {
                     Print("Failed to send pong");
                     pool_managers.QueueMessage(pong_str);
                  }
               } else {
                  Print("Unknown message type: ", msg_type);
               }
            } else {
               Print("Invalid JSON: ", message);
            }
         } else if (out[i].MessageType() == PING_FRAME) {
            Print("Received ping frame");
            ws.SendPong(out[i].ToString());
         } else if (out[i].MessageType() == PONG_FRAME) {
            Print("Received pong frame");
         } else if (out[i].MessageType() == CLOSE_FRAME) {
            Print("Received close frame: ", out[i].ToString());
            ws.Close(NORMAL_CLOSE, "Received close frame from server");
         } else {
            Print("Unsupported frame type: ", EnumToString(out[i].MessageType()));
         }
      }
   }

   void HandlePingAndReconnect(CTradePoolManager &pool_managers) {
      datetime current_time = TimeCurrent();
      if (current_time - last_ping_sent >= InpPingInterval) {
         CJAVal ping_json;
         ping_json["type"] = "ping";
         ping_json["timestamp"] = (long)current_time;
         string ping_str = ping_json.Serialize();
         if (ping_str != "" && SendMessage(ping_str)) {
            Print("Sent ping: ", ping_str);
         } else {
            Print("Failed to send ping");
            pool_managers.QueueMessage(ping_str);
         }
         last_ping_sent = current_time;
      }

      if (ws.ClientState() != CONNECTED) {
         int backoff = InpReconnectDelay * (1 << reconnect_attempts);
         if (backoff > 30) backoff = 30;
         if (current_time - last_reconnect_attempt < backoff) return;

         if (reconnect_attempts >= InpMaxReconnectAttempts) {
            Print("Max reconnection attempts reached (", InpMaxReconnectAttempts, "). Halting reconnection.");
            Alert("WebSocket connection failed. Please check server or restart EA.");
            return;
         }

         last_reconnect_attempt = current_time;
         reconnect_attempts++;
         Print("Connection lost, attempting reconnect (attempt ", reconnect_attempts, ")...");

         if (ws.Connect(InpWebSocketURL, InputPort, 10000)) {
            if (SendHandshake()) {
               Print("Successfully reconnected to ", InpWebSocketURL);
               reconnect_attempts = 0;
               last_reconnect_attempt = 0;
               for (int i = ArraySize(pool_managers.message_queue) - 1; i >= 0; i--) {
                  if (SendMessage(pool_managers.message_queue[i])) {
                     Print("Resent queued message: ", pool_managers.message_queue[i]);
                     ArrayRemove(pool_managers.message_queue, i, 1);
                  }
               }
            } else {
               ws.Close();
            }
         } else {
            Print("Failed to reconnect to ", InpWebSocketURL);
         }
      }
   }

   void Deinitialize() {
      if (ws.ClientState() == CONNECTED) {
         CJAVal disconnect_json;
         disconnect_json["type"] = "disconnect";
         disconnect_json["reason"] = "EA shutdown";
         disconnect_json["timestamp"] = (long)TimeCurrent();
         string json_str = disconnect_json.Serialize();
         if (json_str != "") SendMessage(json_str);
         ws.Close();
      }
   }
};

// Global instances
CTradePoolManager pool_manager;
CWebSocketManager ws_manager;
CWebSocketClient ws;

// Initialization
int OnInit() {
   if (!ws_manager.Initialize(InpWebSocketURL, InputPort)) return INIT_FAILED;
   if (!EventSetMillisecondTimer(InpTimerInterval)) {
      Print("Failed to set timer. Error code: ", GetLastError());
      ws_manager.Deinitialize();
      return INIT_FAILED;
   }
   Print("Trade Pool EA initialized. Connected to ", InpWebSocketURL);
   return INIT_SUCCEEDED;
}

// Timer event
void OnTimer() {
   ws_manager.HandlePingAndReconnect(pool_manager);
   ws_manager.ProcessMessages(pool_manager);
   pool_manager.CleanupTradePool();
}

// Tick event
void OnTick() {
   pool_manager.ProcessTick();
}

// Deinitialization
void OnDeinit(const int reason) {
   EventKillTimer();
   ws_manager.Deinitialize();
   Print("Trade Pool EA deinitialized. Reason: ", reason);
}