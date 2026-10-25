#include <stdarg.h>
#include <stdio.h>
#include <string.h>

#include "pico/stdlib.h"

#include "cec-frame.h"
#include "cec-log.h"
#include "hardware/sync.h"

TaskHandle_t xCECTask;

static cec_message_t rx_message = {.data = {0x0}, .len = 0};
static cec_frame_t rx_frame = {.message = &rx_message};

/* CEC statistics. */
static cec_frame_stats_t cec_stats;
static alarm_id_t rx_ack_alarm = 0;
static bool rx_listening = false;
static volatile uint64_t rx_started = 0;
static volatile uint64_t last_edge = 0;
static volatile cec_tx_result_t tx_result;
static cec_frame_t tx_frame;
static cec_message_t tx_message;


/**
 * Calculate next offset as time since boot.
 */
static uint64_t time_next(uint64_t start, uint64_t next) {
  uint64_t elapsed = time_us_64() - start;
  return elapsed < next ? next - elapsed : 1;
}

/**
 * Pull the CEC line high at the specified time.
 */
static int64_t ack_high(alarm_id_t alarm, void *user_data) {
  gpio_set_dir(CEC_PIN, GPIO_IN);

  return 0;
}

static void frame_rx_isr(uint gpio, uint32_t events) {
  last_edge = time_us_64();
  uint64_t low_time = 0;
  gpio_acknowledge_irq(gpio, events);
  gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_RISE | GPIO_IRQ_EDGE_FALL, false);
  // printf("state = %d, byte = %d, bit = %d\n", rx_frame.state, rx_frame.byte, rx_frame.bit);
  switch (rx_frame.state) {
    case CEC_FRAME_STATE_START_LOW:
      rx_frame.start = time_us_64();
      rx_started = rx_frame.start;
      rx_frame.state = CEC_FRAME_STATE_START_HIGH;
      gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_RISE, true);
      return;
    case CEC_FRAME_STATE_START_HIGH:
      low_time = time_us_64() - rx_frame.start;
      if (low_time >= 3500 && low_time <= 3900) {
        rx_frame.first = true;
        rx_frame.byte = 0;
        rx_frame.bit = 0;
        rx_frame.state = CEC_FRAME_STATE_DATA_LOW;
        gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_FALL, true);
      } else {
        rx_frame.state = CEC_FRAME_STATE_ABORT;
        xTaskNotifyIndexedFromISR(xCECTask, NOTIFY_RX, NOTIFY_RX_ABORT, eSetBits, NULL);
      }
      return;
    case CEC_FRAME_STATE_EOM_LOW:
      rx_frame.byte++;
      rx_frame.bit = 0;
    case CEC_FRAME_STATE_DATA_LOW: {
      uint64_t min_time = rx_frame.first ? 4300 : 2050;
      uint64_t max_time = rx_frame.first ? 4700 : 2750;
      uint64_t bit_time = time_us_64() - rx_frame.start;
      if (bit_time >= min_time && bit_time <= max_time) {
        rx_frame.start = time_us_64();
        if (rx_frame.state == CEC_FRAME_STATE_EOM_LOW) {
          rx_frame.state = CEC_FRAME_STATE_EOM_HIGH;
        } else {
          rx_frame.state = CEC_FRAME_STATE_DATA_HIGH;
        }
        rx_frame.first = false;
        gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_RISE, true);
      } else {
        rx_frame.state = CEC_FRAME_STATE_ABORT;
        xTaskNotifyIndexedFromISR(xCECTask, NOTIFY_RX, NOTIFY_RX_ABORT, eSetBits, NULL);
      }
    }
      return;
    case CEC_FRAME_STATE_EOM_HIGH:
    case CEC_FRAME_STATE_DATA_HIGH:
      low_time = time_us_64() - rx_frame.start;
      uint8_t bit = false;
      if (low_time >= 400 && low_time <= 800) {
        bit = true;
      } else if (low_time >= 1300 && low_time <= 1700) {
        bit = false;
      } else {
        rx_frame.state = CEC_FRAME_STATE_ABORT;
        xTaskNotifyIndexedFromISR(xCECTask, NOTIFY_RX, NOTIFY_RX_ABORT, eSetBits, NULL);
        return;
      }
      if (rx_frame.state == CEC_FRAME_STATE_EOM_HIGH) {
        rx_frame.eom = bit;
        rx_frame.state = CEC_FRAME_STATE_ACK_LOW;
      } else {
        if (rx_frame.byte >= CEC_FRAME_MAX_LEN) {
          rx_frame.state = CEC_FRAME_STATE_ABORT;
          xTaskNotifyIndexedFromISR(xCECTask, NOTIFY_RX, NOTIFY_RX_ABORT, eSetBits, NULL);
          return;
        }
        rx_frame.message->data[rx_frame.byte] <<= 1;
        rx_frame.message->data[rx_frame.byte] |= bit ? 0x01 : 0x00;
        rx_frame.bit++;
        if (rx_frame.bit > 7) {
          rx_frame.state = CEC_FRAME_STATE_EOM_LOW;
        } else {
          rx_frame.state = CEC_FRAME_STATE_DATA_LOW;
        }
      }
      gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_FALL, true);
      return;
    case CEC_FRAME_STATE_ACK_LOW:
      rx_frame.start = time_us_64();
      // send ack by changing ack from 1 to 0
      uint8_t tgt_addr = rx_frame.message->data[0] & 0x0f;
      if ((tgt_addr != 0x0f) && (tgt_addr == rx_frame.address)) {
        rx_frame.state = CEC_FRAME_STATE_ACK_END;
        gpio_set_dir(CEC_PIN, GPIO_OUT);  // pull low, then schedule pull high
        rx_ack_alarm = add_alarm_at(from_us_since_boot(rx_frame.start + 1500), ack_high, NULL, true);
        if (rx_ack_alarm <= 0) gpio_set_dir(CEC_PIN, GPIO_IN);
        rx_frame.ack = true;
      }
      rx_frame.state = CEC_FRAME_STATE_ACK_HIGH;
      gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_RISE, true);
      return;
    case CEC_FRAME_STATE_ACK_HIGH:
      low_time = time_us_64() - rx_frame.start;
      if ((low_time >= 400 && low_time <= 800) || (low_time >= 1300 && low_time <= 1700)) {
        rx_frame.state = CEC_FRAME_STATE_ACK_END;
      } else {
        rx_frame.state = CEC_FRAME_STATE_ABORT;
        xTaskNotifyIndexedFromISR(xCECTask, NOTIFY_RX, NOTIFY_RX_ABORT, eSetBits, NULL);
        return;
      }
      // fall through
    case CEC_FRAME_STATE_ACK_END:
      if (rx_frame.eom) {
        rx_frame.state = CEC_FRAME_STATE_END;
      } else {
        rx_frame.state = CEC_FRAME_STATE_DATA_LOW;
        gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_FALL, true);
        return;
      }
      // finish receiving frame
    case CEC_FRAME_STATE_END:
    default:
      rx_frame.message->len = rx_frame.byte;
      xTaskNotifyIndexedFromISR(xCECTask, NOTIFY_RX, NOTIFY_RX_RX, eSetBits, NULL);
  }
}

static void stop_rx(void) {
  // All CEC task and alarm handlers run on core 0.
  uint32_t irq = save_and_disable_interrupts();
  gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_RISE | GPIO_IRQ_EDGE_FALL, false);
  if (rx_ack_alarm > 0) cancel_alarm(rx_ack_alarm);
  rx_ack_alarm = 0;
  gpio_set_dir(CEC_PIN, GPIO_IN);
  rx_listening = false;
  restore_interrupts(irq);
}

bool cec_frame_receiving(void) {
  return rx_listening && rx_frame.state != CEC_FRAME_STATE_START_LOW &&
         rx_frame.state != CEC_FRAME_STATE_END && rx_frame.state != CEC_FRAME_STATE_ABORT;
}

bool cec_frame_line_high(void) { return gpio_get(CEC_PIN); }

void cec_frame_recv_timeout(cec_message_t *message, uint8_t address, TickType_t wait) {
  message->len = 0;
  if (!rx_listening) {
    uint32_t irq = save_and_disable_interrupts();
    rx_frame.address = address;
    rx_frame.state = CEC_FRAME_STATE_START_LOW;
    rx_frame.ack = false;
    rx_started = 0;
    memset(rx_frame.message, 0, sizeof(*rx_frame.message));
    gpio_set_irq_callback(&frame_rx_isr);
    gpio_acknowledge_irq(CEC_PIN, GPIO_IRQ_EDGE_RISE | GPIO_IRQ_EDGE_FALL);
    rx_listening = true;
    gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_FALL, true);
    restore_interrupts(irq);
  }
  uint32_t notify = 0;
  xTaskNotifyWaitIndexed(NOTIFY_RX, 0, 0xffffffff, &notify, wait);
  if (notify & NOTIFY_RX_RX) {
    stop_rx();
    *message = rx_message;
    cec_log_frame(&rx_frame, true);
    ++cec_stats.rx_frames;
  } else if ((notify & (NOTIFY_RX_ABORT | NOTIFY_RX_TX)) ||
             (rx_started && time_us_64() - rx_started > 500000)) {
    stop_rx();
    ++cec_stats.rx_abort_frames;
  }
}

void cec_frame_recv(cec_message_t *message, uint8_t address) {
  cec_frame_recv_timeout(message, address, pdMS_TO_TICKS(50));
}

static int64_t tx_stop(cec_tx_result_t result) {
  gpio_set_dir(CEC_PIN, GPIO_IN);
  tx_result = result;
  vTaskNotifyGiveIndexedFromISR(xCECTask, NOTIFY_TX, NULL);
  return 0;
}

static int64_t frame_tx_callback(alarm_id_t alarm, void *user_data) {
  cec_frame_t *frame = user_data;
  uint64_t low_time;
  switch (frame->state) {
    case CEC_FRAME_STATE_START_LOW:
      gpio_set_dir(CEC_PIN, GPIO_OUT);
      frame->start = time_us_64();
      frame->state = CEC_FRAME_STATE_START_HIGH;
      return time_next(frame->start, 3700);
    case CEC_FRAME_STATE_START_HIGH:
      gpio_set_dir(CEC_PIN, GPIO_IN);
      frame->state = CEC_FRAME_STATE_DATA_LOW;
      return time_next(frame->start, 4500);
    case CEC_FRAME_STATE_DATA_LOW:
      gpio_set_dir(CEC_PIN, GPIO_OUT);
      frame->start = time_us_64();
      low_time = (frame->message->data[frame->byte] & (1 << frame->bit)) ? 600 : 1500;
      frame->state = CEC_FRAME_STATE_DATA_HIGH;
      return time_next(frame->start, low_time);
    case CEC_FRAME_STATE_DATA_HIGH:
      gpio_set_dir(CEC_PIN, GPIO_IN);
      frame->state = CEC_FRAME_STATE_DATA_SAMPLE;
      low_time = (frame->message->data[frame->byte] & (1 << frame->bit)) ? 1050 : 1800;
      return time_next(frame->start, low_time);
    case CEC_FRAME_STATE_DATA_SAMPLE:
      // A transmitted 1 overridden by another initiator is a lost arbitration.
      if ((frame->message->data[frame->byte] & (1 << frame->bit)) && !gpio_get(CEC_PIN))
        return tx_stop(CEC_TX_COLLISION);
      if (frame->bit--) frame->state = CEC_FRAME_STATE_DATA_LOW;
      else { ++frame->byte; frame->state = CEC_FRAME_STATE_EOM_LOW; }
      return time_next(frame->start, 2400);
    case CEC_FRAME_STATE_EOM_LOW:
      gpio_set_dir(CEC_PIN, GPIO_OUT);
      frame->start = time_us_64();
      frame->state = CEC_FRAME_STATE_EOM_HIGH;
      return time_next(frame->start, frame->byte < frame->message->len ? 1500 : 600);
    case CEC_FRAME_STATE_EOM_HIGH:
      gpio_set_dir(CEC_PIN, GPIO_IN);
      frame->state = CEC_FRAME_STATE_ACK_LOW;
      return time_next(frame->start, 2400);
    case CEC_FRAME_STATE_ACK_LOW:
      gpio_set_dir(CEC_PIN, GPIO_OUT);
      frame->start = time_us_64();
      frame->state = CEC_FRAME_STATE_ACK_HIGH;
      return time_next(frame->start, 600);
    case CEC_FRAME_STATE_ACK_HIGH:
      gpio_set_dir(CEC_PIN, GPIO_IN);
      frame->state = CEC_FRAME_STATE_ACK_WAIT;
      return time_next(frame->start, 1050);
    case CEC_FRAME_STATE_ACK_WAIT:
      frame->ack = (frame->message->header & 15) == 15 ? gpio_get(CEC_PIN) : !gpio_get(CEC_PIN);
      frame->state = CEC_FRAME_STATE_ACK_END;
      return time_next(frame->start, 2400);
    case CEC_FRAME_STATE_ACK_END:
      if (!frame->ack) return tx_stop(CEC_TX_NACK);
      if (frame->byte == frame->message->len) return tx_stop(CEC_TX_ACK);
      frame->bit = 7;
      frame->state = CEC_FRAME_STATE_DATA_LOW;
      return 1;
    default: return tx_stop(CEC_TX_ERROR);
  }
}

cec_tx_result_t cec_frame_send_result(const cec_message_t *message, uint32_t timeout_ms) {
  if (!message || !message->len || message->len > CEC_FRAME_MAX_LEN) return CEC_TX_ERROR;
  stop_rx();
  if (!timeout_ms) return CEC_TX_TIMEOUT;
  uint64_t deadline = time_us_64() + (uint64_t)timeout_ms * 1000;
  // Start as soon as the line is released; bit-level arbitration below still
  // detects a real collision.
  while (!gpio_get(CEC_PIN)) {
    if (time_us_64() >= deadline) return CEC_TX_BUSY;
    vTaskDelay(1);
  }
  // Static storage plus cancellation prevents an alarm retaining a stack pointer.
  tx_message = *message;
  tx_frame = (cec_frame_t){.message = &tx_message, .bit = 7, .state = CEC_FRAME_STATE_START_LOW};
  tx_result = CEC_TX_TIMEOUT;
  ulTaskNotifyTakeIndexed(NOTIFY_TX, pdTRUE, 0);
  alarm_id_t alarm = add_alarm_in_us(1, frame_tx_callback, &tx_frame, true);
  if (alarm <= 0) { gpio_set_dir(CEC_PIN, GPIO_IN); return CEC_TX_ERROR; }
  uint64_t now = time_us_64();
  TickType_t wait = now < deadline ? pdMS_TO_TICKS((deadline - now) / 1000) : 0;
  uint32_t done = ulTaskNotifyTakeIndexed(NOTIFY_TX, pdTRUE, wait);
  uint32_t irq = save_and_disable_interrupts();
  cancel_alarm(alarm);
  gpio_set_dir(CEC_PIN, GPIO_IN);
  if (!done) tx_result = CEC_TX_TIMEOUT;
  restore_interrupts(irq);
  cec_log_frame(&tx_frame, false);
  if (tx_result == CEC_TX_ACK) ++cec_stats.tx_frames;
  else ++cec_stats.tx_noack_frames;
  return tx_result;
}

bool cec_frame_send(const cec_message_t *message) {
  return cec_frame_send_result(message, 500) == CEC_TX_ACK;
}

void cec_frame_get_stats(cec_frame_stats_t *stats) {
  *stats = cec_stats;
}

void cec_frame_init(void) {
  gpio_init(CEC_PIN);
  gpio_disable_pulls(CEC_PIN);
  gpio_put(CEC_PIN, 0);
  gpio_set_dir(CEC_PIN, GPIO_IN);

  gpio_set_irq_callback(&frame_rx_isr);
  irq_set_enabled(IO_IRQ_BANK0, true);
  gpio_set_irq_enabled(CEC_PIN, GPIO_IRQ_EDGE_RISE | GPIO_IRQ_EDGE_FALL, false);
}
