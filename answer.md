Sem pressa, vamos por partes. Antes de entrar no código, três conceitos que aparecem o tempo todo e que preciso que fiquem claros, porque tudo mais é só combinação deles:

- goroutine (go func() {...}()): é como mandar alguém fazer uma tarefa "em paralelo", sem travar o que você tava fazendo. Você "dispara e esquece" — quem chamou continua a vida dele.
- channel (chan): é uma "caixinha de correio" entre goroutines. Uma goroutine põe uma carta (updates <- valor), outra tira a carta (<-updates). É a forma seguros de uma goroutine "avisar" a outra sem elas mexerem direto na mesma variável ao mesmo tempo (o que geraria bug).
- context (ctx): é um "controle remoto de desligar". Você passa ele adiante pra qualquer goroutine que precise saber "pare o que você tá fazendo". Quando alguém chama cancel(), todo mundo que tá "escutando" aquele ctx (via ctx.Done()) percebe e para.

Guarda isso e vamos pro código.

internal/docker/stats.go — a "torneira" de dados

Pensa assim: o Docker, se você pedir, fica te mandando dados novos de um container de tempo em tempo, tipo uma torneira pingando — uma gota (amostra) por segundo, sozinho, sem você precisar ficar perguntando "e agora? e agora?".

StreamStats é a função que abre essa torneira pra um container específico e, a cada gota que cai, empacota ela bonitinho e joga na "caixinha de correio" (channel) pra alguém usar:

func (c *Client) StreamStats(ctx context.Context, id string, updates chan<- StatsUpdate) {
    result, err := c.ContainerStats(ctx, id, client.ContainerStatsOptions{Stream: true})
    // "Stream: true" = "Docker, fica me mandando, não manda só uma vez"
    ...
    for {
        // esse for é infinito: fica sempre esperando a próxima gota
        var stats container.StatsResponse
        decoder.Decode(&stats)  // pega a gota (dado cru que o Docker mandou)

        select {
        case updates <- StatsUpdate{...}:  // joga a gota formatada na caixinha
        case <-ctx.Done():                 // OU: "pediram pra parar" — sai do loop
            return
        }
    }
}

O select ali é um "o que acontecer primeiro": ou eu consigo entregar a gota na caixinha, ou alguém apertou o botão de parar (ctx.Done()). O que vier primeiro, é o que acontece — assim a função nunca fica travada pra sempre esperando alguém pegar a carta se ninguém mais tá escutando.

formatStatsText é só a parte "arrumar a gota pra ficar bonita": pega o dado cru do Docker (bytes usados de memória, bytes de rede, etc) e transforma em texto tipo:
CPU:    3.20%
Memory: 45.2MiB / 512.0MiB
...

Isso já existia pra montar a aba Stats uma vez só (quando você seleciona o container). O que eu fiz foi tirar esse "arrumar a gota" de dentro daquela função antiga e transformar numa função separada (formatStatsText), pra tanto a versão "uma vez só" quanto a nova versão "toda hora" (StreamStats) usarem o mesmo código de formatação, sem copiar e colar.

Resumindo o arquivo: antes só existia StreamCPU (torneira que só manda o % de CPU, pras linhas da esquerda). Agora existe também StreamStats (torneira que manda o texto completo — CPU + memória + rede + disco —, pra aba Stats da direita). São duas torneiras independentes, ligadas em containers diferentes ao mesmo tempo.

internal/app/app.go — quem liga a torneira em quem

Esse arquivo é o "eletricista": ele pega as torneiras que o stats.go oferece e liga cada uma no lugar certo da tela.

A parte que interessa é onSelectContainer — a função que roda toda vez que você troca de container selecionado na lista da esquerda:

onSelectContainer := func(id string) {
    selectMu.Lock()
    if selectCancel != nil {
        selectCancel()   // desliga a torneira do container ANTERIOR
    }
    selCtx, selCancel := context.WithCancel(ctx)
    selectCancel = selCancel   // guarda o "botão de desligar" desse novo container
    selectMu.Unlock()

Por quê desligar a anterior? Porque só faz sentido ter uma torneira de stats aberta por vez — a do container que você está olhando agora. Se você troca de container, a torneira do container antigo não serve mais pra nada (e ainda ia ficar gastando recursos e mandando dado que ninguém vai ler).

Depois disso, ele faz duas coisas em paralelo (duas goroutines):

1) Busca única de Logs/Config/Top (isso já existia, não mudou):
go func() {
    info, err := cli.Info(selCtx, id)
    ...
    dashboard.SetContainerInfo(...)
}()

2) A parte nova — abre a torneira de stats e fica ouvindo ela:
statsUpdates := make(chan docker.StatsUpdate, 1)   // cria a caixinha de correio
go cli.StreamStats(selCtx, id, statsUpdates)        // abre a torneira, jogando na caixinha

go func() {                                          // essa goroutine só fica "vigiando o correio"
    for {
        select {
        case u := <-statsUpdates:        // chegou uma gota nova?
            if selCtx.Err() != nil {
                return                    // se enquanto isso já mandaram parar, ignora e sai
            }
            dashboard.UpdateStats(u.Text) // manda pra tela mostrar
        case <-selCtx.Done():             // OU: mandaram parar (trocou de container)
            return
        }
    }
}()

Ou seja: uma goroutine fica só "abrindo a torneira" (StreamStats), e outra fica só "olhando a caixinha de correio e repassando pra tela" (UpdateStats). Elas se comunicam pela channel, e as duas obedecem o mesmo selCtx — então quando você troca de container, o selectCancel() lá em cima desliga as duas de uma vez, sem precisar avisar cada uma na mão.

O truque central do arquivo inteiro, que também explica o Mutex de que falamos antes: cada seleção de container cria seu próprio selCtx/selCancel, e a próxima seleção sempre desliga a anterior antes de ligar a nova. É como trocar de canal na TV — você não deixa o canal antigo continuar tocando som no fundo, você desliga ele e liga o novo.
