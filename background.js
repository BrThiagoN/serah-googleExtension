// background.js - Service Worker da Extensão
// Este script roda em segundo plano e é ativado por eventos do navegador.
// ATENÇÃO: Service workers são efêmeros. Não salve estados em variáveis globais. Use chrome.storage.

// Configura o painel lateral para abrir automaticamente ao clicar no ícone da extensão
chrome.sidePanel
  .setPanelBehavior({ openPanelOnActionClick: true })
  .catch((error) => console.error("Erro ao configurar comportamento do sidePanel:", error));

// Evento disparado quando a extensão é instalada ou atualizada
chrome.runtime.onInstalled.addListener(async (details) => {
  console.log("Extensão instalada com sucesso! Motivo:", details.reason);
  
  // Exemplo de inicialização de estado usando chrome.storage.local
  await chrome.storage.local.set({ extensionActive: true, installTime: Date.now() });
  console.log("Estado inicial configurado no chrome.storage.local.");
});

// Evento para escutar mensagens enviadas do popup.js ou content.js
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  console.log("Mensagem recebida no background.js:", message, "Enviado por:", sender);

  // Exemplo de processamento assíncrono de mensagens
  if (message.action === "obterDadosIniciais") {
    (async () => {
      try {
        const storageData = await chrome.storage.local.get(["extensionActive", "installTime"]);
        
        // Retorna a resposta para quem enviou a mensagem
        sendResponse({ 
          status: "sucesso", 
          dados: storageData 
        });
      } catch (error) {
        console.error("Erro ao ler dados no background:", error);
        sendResponse({ status: "erro", mensagem: error.message });
      }
    })();

    // Retornar true mantém o canal de comunicação aberto para respostas assíncronas
    return true; 
  }

  // Se você tiver outras ações, adicione condições aqui
  // Exemplo:
  /*
  if (message.action === "outraAcao") {
    // Insira seu código aqui...
    sendResponse({ status: "concluido" });
  }
  */
});
