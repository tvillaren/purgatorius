package imap

import (
	"errors"
	"strconv"
	"strings"
)

// Commandes qui modifient la boite : seule la mise a la corbeille les
// emploie (paquet synchro, MettreALaCorbeille). La synchro, elle, reste en
// lecture seule (Examiner, BODY.PEEK).

// Selectionner ouvre un dossier en lecture et ecriture (SELECT).
func (c *Client) Selectionner(nom string) (Etat, error) {
	var et Etat
	_, err := c.commande("SELECT "+citer(c.nomVersServeur(nom)), func(e []noeud) {
		if len(e) >= 2 && strings.EqualFold(e[1].s, "EXISTS") {
			et.Existe, _ = strconv.Atoi(e[0].s)
		}
		if len(e) >= 2 && estStatut(e[0].s) {
			if m := reUIDValidity.FindStringSubmatch(e[1].s); m != nil {
				v, _ := strconv.ParseUint(m[1], 10, 32)
				et.UIDValidity = uint32(v)
			}
		}
	})
	return et, err
}

// GmMsgIDs lit le X-GM-MSGID des messages d'UID dans ensemble, dans le
// dossier ouvert : UID -> X-GM-MSGID. Les UID absents du dossier n'y sont pas.
func (c *Client) GmMsgIDs(ensemble string) (map[uint32]uint64, error) {
	if !c.Capacite("X-GM-EXT-1") {
		return nil, errors.New("extension Gmail X-GM-EXT-1 absente")
	}
	out := map[uint32]uint64{}
	_, err := c.commande("UID FETCH "+ensemble+" (UID X-GM-MSGID)", func(e []noeud) {
		if len(e) >= 3 && strings.EqualFold(e[1].s, "FETCH") && e[2].genre == 'l' && porte(e[2].l, "UID", "X-GM-MSGID") {
			if m := analyserFetch(e[2].l, c.utf8); m.UID != 0 && m.GmMsgID != 0 {
				out[m.UID] = m.GmMsgID
			}
		}
	})
	return out, err
}

// ChercherGmMsgID renvoie les UID du dossier ouvert qui portent ce X-GM-MSGID.
func (c *Client) ChercherGmMsgID(gm uint64) ([]uint32, error) {
	if !c.Capacite("X-GM-EXT-1") {
		return nil, errors.New("extension Gmail X-GM-EXT-1 absente")
	}
	return c.RechercherUID("X-GM-MSGID " + strconv.FormatUint(gm, 10))
}

// Deplacer deplace les messages d'UID dans ensemble vers le dossier vers
// (UID MOVE, RFC 6851). Dans Gmail, deplacer depuis « Tous les messages » vers
// la corbeille met le message a la corbeille.
func (c *Client) Deplacer(ensemble, vers string) error {
	if !c.Capacite("MOVE") {
		return errors.New("extension MOVE absente")
	}
	_, err := c.commande("UID MOVE "+ensemble+" "+citer(c.nomVersServeur(vers)), nil)
	return err
}
