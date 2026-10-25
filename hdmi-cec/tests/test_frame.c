#include <assert.h>
#include <stdio.h>
#include "../firmware/src/cec-frame.c"

static uint64_t clock_us, alarm_due;
static alarm_callback_t pending;
static void *pending_arg;
static unsigned notified[2];
static bool output, stuck_low, missing_timer, collision;
static unsigned nack_byte, canceled, alarm_seq;
static int pending_id;

uint64_t time_us_64(void) { return clock_us; }
void gpio_set_dir(uint p, bool d) { (void)p; output=d; }
bool gpio_get(uint p) {
  (void)p;
  if (output || stuck_low) return false;
  if (pending && tx_frame.state==CEC_FRAME_STATE_ACK_WAIT)
    return tx_frame.byte==nack_byte;
  if (pending && collision && tx_frame.state==CEC_FRAME_STATE_DATA_SAMPLE) return false;
  return true;
}
void gpio_acknowledge_irq(uint p,uint32_t e) { (void)p;(void)e; }
void gpio_set_irq_enabled(uint p,uint32_t e,bool b) {(void)p;(void)e;(void)b;}
void gpio_set_irq_callback(void (*f)(uint,uint32_t)) {(void)f;}
void gpio_init(uint p) {(void)p;}
void gpio_disable_pulls(uint p) {(void)p;}
void gpio_put(uint p,bool b) {(void)p;assert(!b);}
void irq_set_enabled(uint p,bool b) {(void)p;(void)b;}
uint32_t save_and_disable_interrupts(void) {return 0;}
void restore_interrupts(uint32_t v) {(void)v;}
alarm_id_t add_alarm_at(absolute_time_t t,alarm_callback_t f,void *arg,bool b) {
  (void)b; pending=f;pending_arg=arg;alarm_due=t;pending_id=++alarm_seq;return pending_id;
}
alarm_id_t add_alarm_in_us(uint64_t d,alarm_callback_t f,void *arg,bool b) {
  return add_alarm_at(clock_us+d,f,arg,b);
}
bool cancel_alarm(alarm_id_t id) {
  ++canceled;
  if (id==pending_id) {pending=NULL;return true;}
  return false;
}
void vTaskDelay(TickType_t t) {clock_us+=(uint64_t)t*1000;}
void xTaskNotifyIndexedFromISR(TaskHandle_t h,unsigned i,uint32_t v,int a,void *x) {
  (void)h;(void)x;
  if(a==eSetBits) notified[i]|=v; else ++notified[i];
}
void vTaskNotifyGiveIndexedFromISR(TaskHandle_t h,unsigned i,void *x) {
  (void)h;(void)x;++notified[i];
}
uint32_t ulTaskNotifyTakeIndexed(unsigned i,int clear,TickType_t wait) {
  (void)clear;
  uint64_t end=clock_us+(uint64_t)wait*1000;
  while(!notified[i] && pending && !missing_timer && alarm_due<=end) {
    clock_us=alarm_due;
    alarm_callback_t f=pending;
    int64_t again=f(pending_id,pending_arg);
    if(again>0) alarm_due=clock_us+again; else pending=NULL;
  }
  if(!notified[i]) clock_us=end;
  uint32_t n=notified[i];notified[i]=0;return n;
}
int xTaskNotifyWaitIndexed(unsigned i,uint32_t a,uint32_t b,uint32_t *out,TickType_t t) {
  (void)a;(void)b;
  *out=notified[i];notified[i]=0;
  if(!*out) vTaskDelay(t);
  return *out!=0;
}
void cec_log_frame(cec_frame_t *f,bool b) {(void)f;(void)b;}

static void reset(void) {
  clock_us=0;pending=NULL;output=false;stuck_low=false;missing_timer=false;collision=false;
  nack_byte=0;canceled=0;notified[0]=notified[1]=0;rx_listening=false;rx_ack_alarm=0;
}

static void edge_at(uint64_t t, uint32_t event) {
  clock_us=t;
  frame_rx_isr(CEC_PIN,event);
}
static void receive_bytes(const uint8_t *bytes, unsigned count) {
  cec_message_t rx={0};
  cec_frame_recv_timeout(&rx,4,0);
  edge_at(1000,GPIO_IRQ_EDGE_FALL);
  edge_at(4700,GPIO_IRQ_EDGE_RISE);
  uint64_t t=5500;
  for(unsigned n=0;n<count;n++) {
    for(int bit=7;bit>=0;bit--) {
      edge_at(t,GPIO_IRQ_EDGE_FALL);
      edge_at(t+((bytes[n]&(1<<bit))?600:1500),GPIO_IRQ_EDGE_RISE);
      t+=2400;
    }
    edge_at(t,GPIO_IRQ_EDGE_FALL);
    edge_at(t+(n+1==count?600:1500),GPIO_IRQ_EDGE_RISE);
    t+=2400;
    edge_at(t,GPIO_IRQ_EDGE_FALL);
    if(pending) { clock_us=t+1500; pending(pending_id,pending_arg); pending=NULL; }
    edge_at(t+1500,GPIO_IRQ_EDGE_RISE);
    t+=2400;
  }
  cec_frame_recv_timeout(&rx,4,0);
  assert(rx.len==count && memcmp(rx.data,bytes,count)==0);
}

int main(void) {
  cec_message_t m={.header=0x40,.opcode=4,.len=2};
  reset();
  assert(cec_frame_send_result(&m,500)==CEC_TX_ACK);
  assert(!output && !pending && canceled>0 && clock_us<500000);
  reset();nack_byte=1;
  assert(cec_frame_send_result(&m,500)==CEC_TX_NACK);
  assert(tx_frame.byte==1 && !pending && !output); // abort on header NACK
  reset();nack_byte=2;
  assert(cec_frame_send_result(&m,500)==CEC_TX_NACK);
  reset();stuck_low=true;
  assert(cec_frame_send_result(&m,50)==CEC_TX_BUSY);
  assert(clock_us<=50000 && !pending && !output);
  reset();missing_timer=true;
  assert(cec_frame_send_result(&m,50)==CEC_TX_TIMEOUT);
  assert(!pending && !output && canceled>0);
  reset();collision=true;
  assert(cec_frame_send_result(&m,500)==CEC_TX_COLLISION);
  assert(!pending && !output);
  reset();
  assert(cec_frame_send_result(&m,0)==CEC_TX_TIMEOUT);
  assert(!output && !pending);
  reset();
  cec_message_t rx={0};
  cec_frame_recv_timeout(&rx,4,10);
  assert(rx_listening && !rx.len);
  rx_frame.state=CEC_FRAME_STATE_DATA_LOW;
  rx_started=1;
  clock_us=510000;
  cec_frame_recv_timeout(&rx,4,10);
  assert(!rx_listening && !output);
  // Cancellation releases a line held by an outstanding receiver ACK timer.
  reset();rx_listening=true;output=true;
  rx_ack_alarm=add_alarm_in_us(1500,ack_high,NULL,true);
  stop_rx();
  assert(!pending && !output && canceled>0);
  reset();
  const uint8_t report[]={0x04,0x90,0x00};
  receive_bytes(report,sizeof(report));
  reset();
  // Reject a 17th byte before touching the 16-byte receive buffer.
  rx_frame.state=CEC_FRAME_STATE_DATA_HIGH;
  rx_frame.byte=CEC_FRAME_MAX_LEN;
  rx_frame.start=100;
  edge_at(700,GPIO_IRQ_EDGE_RISE);
  assert(rx_frame.state==CEC_FRAME_STATE_ABORT);
  puts("frame timeout / ACK / collision / cleanup scenarios passed");
}
